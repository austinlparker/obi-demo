//! Notifier: subscribes to `orders.placed` on NATS, "emails" the customer,
//! and tells the catalog how much stock was sold. No OpenTelemetry crates:
//! OBI captures the outgoing HTTP calls, and the notifier forwards checkout's
//! traceparent onto them so they join the order's trace.

use std::{env, time::Duration};

use axum::{Router, routing::get};
use futures::StreamExt;
use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
struct Item {
    sku: String,
    quantity: i32,
}

#[derive(Debug, Deserialize)]
struct Order {
    order_id: String,
    user_id: String,
    total_cents: i64,
    items: Vec<Item>,
}

#[derive(Serialize)]
struct Sold {
    quantity: i32,
}

/// The trace ID and flags of the W3C traceparent that checkout forwards in the
/// NATS message headers. OBI can't carry trace context through the broker, so
/// checkout and the notifier pass it along by hand.
struct TraceContext {
    trace_id: String,
    flags: String,
}

impl TraceContext {
    fn from_headers(headers: Option<&async_nats::HeaderMap>) -> Option<Self> {
        let value = headers?.get("traceparent")?.as_str();
        let lower_hex = |s: &str, len| {
            s.len() == len && s.bytes().all(|b| matches!(b, b'0'..=b'9' | b'a'..=b'f'))
        };
        let mut parts = value.split('-');
        let (version, trace_id, parent_id, flags) =
            (parts.next()?, parts.next()?, parts.next()?, parts.next()?);
        if version != "00"
            || !lower_hex(trace_id, 32)
            || !lower_hex(parent_id, 16)
            || !lower_hex(flags, 2)
            || parts.next().is_some()
        {
            return None;
        }
        Some(Self { trace_id: trace_id.to_string(), flags: flags.to_string() })
    }

    /// A traceparent for one outgoing request. The span ID must be new: OBI
    /// adopts a traceparent it finds on an outgoing HTTP/1 request as its client
    /// span's own context, so reusing checkout's span ID would give two spans
    /// the same ID.
    fn child(&self) -> String {
        let span_id = rand::random::<u64>().max(1);
        format!("00-{}-{span_id:016x}-{}", self.trace_id, self.flags)
    }
}

fn env_or(key: &str, default: &str) -> String {
    env::var(key).unwrap_or_else(|_| default.to_string())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let nats_url = env_or("NATS_URL", "nats://nats:4222");
    let subject = env_or("ORDERS_SUBJECT", "orders.placed");
    let catalog_url = env_or("CATALOG_URL", "http://catalog:8000");
    let port = env_or("PORT", "7070");

    // Small health endpoint. It also gives OBI a listening port to select
    // this process by.
    let app = Router::new().route("/healthz", get(|| async { "ok" }));
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{port}")).await?;
    tokio::spawn(async move { axum::serve(listener, app).await });

    let nats = loop {
        match async_nats::connect(&nats_url).await {
            Ok(c) => break c,
            Err(e) => {
                eprintln!("waiting for NATS at {nats_url}: {e}");
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
        }
    };
    let mut orders = nats.subscribe(subject.clone()).await?;
    let http = reqwest::Client::builder().timeout(Duration::from_secs(5)).build()?;
    println!("notifier listening on :{port}, subscribed to {subject}");

    while let Some(msg) = orders.next().await {
        let order: Order = match serde_json::from_slice(&msg.payload) {
            Ok(o) => o,
            Err(e) => {
                eprintln!("bad order payload: {e}");
                continue;
            }
        };
        println!(
            "order {} for {}: ${:.2}, emailing receipt",
            order.order_id,
            order.user_id,
            order.total_cents as f64 / 100.0
        );
        let trace = TraceContext::from_headers(msg.headers.as_ref());
        for item in &order.items {
            let url = format!("{catalog_url}/products/{}/sold", item.sku);
            let mut req = http.post(&url).json(&Sold { quantity: item.quantity });
            if let Some(trace) = &trace {
                req = req.header("traceparent", trace.child());
            }
            match req.send().await {
                Ok(r) if r.status().is_success() => {}
                Ok(r) => eprintln!("catalog said {} for {}", r.status(), item.sku),
                Err(e) => eprintln!("catalog call failed for {}: {e}", item.sku),
            }
        }
    }
    Ok(())
}
