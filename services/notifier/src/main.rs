//! Notifier: subscribes to `orders.placed` on NATS, "emails" the customer,
//! and tells the catalog how much stock was sold. No OpenTelemetry crates:
//! OBI captures the NATS consumer and the outgoing HTTP calls.

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
        for item in &order.items {
            let url = format!("{catalog_url}/products/{}/sold", item.sku);
            match http.post(&url).json(&Sold { quantity: item.quantity }).send().await {
                Ok(r) if r.status().is_success() => {}
                Ok(r) => eprintln!("catalog said {} for {}", r.status(), item.sku),
                Err(e) => eprintln!("catalog call failed for {}: {e}", item.sku),
            }
        }
    }
    Ok(())
}
