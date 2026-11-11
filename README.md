# OBI demo: a five-language swag shop with zero instrumentation code

A small shop written in Go, Python, Node.js, Java and Rust, observed entirely by
[OpenTelemetry eBPF Instrumentation (OBI) v1.0.0](https://opentelemetry.io/blog/2026/obi-v1-release/).
None of the services has an OpenTelemetry SDK, agent or library. OBI watches them
from the kernel and sends traces and metrics through an OpenTelemetry Collector
to **Jaeger** and **Prometheus**, with **Perses** dashboards defined as code.

It runs on a Mac inside one Lima Linux VM.

```
                k6 load generator
                       │ HTTP
                       ▼
                ┌─────────────┐   gRPC    ┌──────────────┐  NATS   ┌──────────────┐
  browser ────▶ │ gateway (Go)│─────────▶ │checkout (Java)│───────▶ │notifier (Rust)│
                └─────────────┘           └──────────────┘         └──────────────┘
                  │ HTTP    │ HTTP                                       │ HTTP
                  ▼         ▼                                            ▼
        ┌────────────────┐ ┌──────────────┐                    (catalog: stock sold)
        │catalog (Python)│ │ cart (Node.js)│
        └────────────────┘ └──────────────┘
                │ SQL            │ RESP
             Postgres          Redis

  OBI (eBPF, host PID + host network) ──OTLP──▶ OTel Collector ──▶ Jaeger (traces)
                                                               └──▶ Prometheus (metrics) ◀── Perses
```

## What it shows

- **Traces across five runtimes with no SDKs.** OBI propagates W3C `traceparent`
  itself (`context_propagation: all`): network-level for HTTP/1 and plaintext
  HTTP/2 and gRPC, plus Go library-level propagation in the gateway. Across
  NATS, where OBI can't propagate, checkout and the notifier forward the
  `traceparent` header themselves, with no OpenTelemetry library.
- **Protocol parsing.** PostgreSQL (catalog), Redis (cart), gRPC (gateway to
  checkout) and NATS publishes (checkout) all show up as client spans and
  metrics, parsed from the wire. The notifier's NATS deliveries don't: see
  [Troubleshooting](#troubleshooting).
- **Header and body capture.** OBI records selected HTTP headers and the small
  POST bodies as span attributes, with cookies obfuscated. The gateway sets an
  `X-Order-Id` response header on checkout, so in Jaeger you can search
  `http.response.header.x-order-id=ord-…` to find any order's trace. The
  rules are in `config/obi/obi.yaml`.
- **Route discovery.** FastAPI routes come from OBI's framework route
  discovery. The other services use the route patterns in
  `config/obi/obi.yaml`, with heuristic grouping for anything unmatched.
- **RED metrics, service graph, runtime and network metrics.** Runtime metrics
  for Go, Java and Node.js are experimental in v1.
- **Config v2.** `config/obi/obi.yaml` uses the new OBI Config v2 schema, in the
  OpenTelemetry declarative configuration shape, and passes `obi config validate`.
- **Native histograms and exemplars.** The Collector's spanmetrics connector
  turns OBI's spans into exponential histograms with exemplars. Prometheus stores
  them as native histograms, and the Perses histograms dashboard shows a latency
  heatmap and p95 lines whose exemplars carry the trace ID.
- **Fault injection.** `make chaos-latency` and `make chaos-errors` let you break
  things live and watch it in Perses and Jaeger.

## Requirements

- macOS on Apple silicon or Intel.
- [Lima](https://lima-vm.io) 1.0 or newer: `brew install lima`.
- About 8 GB of RAM and 40 GB of disk for the VM (both set in `lima/obi-demo.yaml`).

### Why a Lima VM and not Apple's `container` tool

OBI is Linux-only eBPF. It needs a kernel with BTF, root or the right
capabilities, and **the same kernel as the processes it instruments**, because
it attaches to them with `pid: host`.

Apple's `container` tool (the Containerization framework) runs every container
in its own lightweight VM with its own kernel. An OBI container there can only
see its own processes, never the shop's. So the demo uses one Linux VM on
Apple's Virtualization framework (Lima `vmType: vz`) and runs rootful Docker
Engine inside it. The containers are still native arm64 Linux containers; they
just share one kernel, which is what OBI needs.

## Run it

```sh
make vm      # first time only: creates the Lima VM (Ubuntu 24.04, Docker Engine)
make up      # builds the five services and starts everything
make open    # opens the shop, Perses and Jaeger
```

The first `make up` builds five images and takes a few minutes. Then:

| What | URL |
| --- | --- |
| Shop (and its API) | http://localhost:8080 |
| Perses dashboards | http://localhost:8090 (project **Swag shop**) |
| Jaeger | http://localhost:16686 |
| Prometheus | http://localhost:9090 |

k6 starts sending about 8 shopping sessions a second as soon as the gateway is up.
Give OBI a minute to discover the processes and for the first metrics to land.

Break things and watch:

```sh
make chaos-latency   # catalog gets ~400 ms slower
make chaos-errors    # 20% of cart requests fail
make chaos-off
```

Checkout also declines about 3% of cards on its own (`CHECKOUT_DECLINE_RATE`),
which shows up as gRPC `FAILED_PRECONDITION` and HTTP 402 at the gateway.

Other targets: `make ps`, `make logs`, `make obi-logs`, `make validate`,
`make check-kernel`, `make down`, `make vm-stop`, `make vm-delete`. Run `make`
for the list.

### On a Linux host

Skip the VM. With Docker Engine (rootful) and a BTF kernel (5.8+), the same
`make up` runs Docker directly.

## How OBI is set up

`docker-compose.yaml` runs `otel/ebpf-instrument:v1.0.0` with:

- `pid: host` and `network_mode: host`, so it sees every process and network
  interface in the VM. Host networking is also why OBI reaches the Collector on
  `127.0.0.1:4318`, which the Collector publishes on the VM's loopback only.
- `privileged: true` for simplicity. The capabilities this config actually
  needs are listed at the top of `config/obi/obi.yaml`.
- `/sys/fs/cgroup`, `/sys/kernel/tracing` and `/sys/kernel/security` mounted,
  which context propagation needs on current kernels.

`config/obi/obi.yaml` selects the five services by the port each listens on
(8088, 8000, 3000, 50051, 7070) and excludes everything else. Service names come
from `OTEL_SERVICE_NAME` in each container's environment, which OBI reads from
the instrumented process.

The Collector (`config/otelcol/config.yaml`) forwards traces to Jaeger over
OTLP/gRPC and metrics to Prometheus' native OTLP endpoint
(`--web.enable-otlp-receiver`). Prometheus turns `service.name` into the `job`
label, so `job="catalog"` is the catalog service.

## Perses dashboards as code

Everything Perses loads lives in `config/perses/provisioning/`:

- `project.yaml`: the **Swag shop** project.
- `datasources.yaml`: Prometheus and Jaeger, proxied by the Perses server.
- `dashboard-services.yaml`: golden signals per service, latency and status by
  route, outgoing HTTP/gRPC, database and NATS calls, the service graph, and a
  table of recent traces from Jaeger. Clicking a trace name opens it in the
  Jaeger UI at localhost:16686.
- `dashboard-runtime-network.yaml`: Go, JVM and Node.js runtime metrics, network
  flows and TCP stats.
- `dashboard-histograms.yaml`: latency heatmap, distribution and quantiles from
  native histograms, and p95 per endpoint with exemplars. Hover an exemplar
  diamond for its `trace_id` and paste it into Jaeger's trace ID search.

### Where the exemplars come from

OBI v1's OTLP metrics don't carry exemplars. Its `exemplar_filter` setting
exists only for its Prometheus exporter, and the Config v2 schema rejects
`meter_provider.exemplar_filter`. So the Collector's `spanmetrics` connector
builds latency histograms from OBI's spans instead (exponential buckets,
exemplars on), and Prometheus runs with `--enable-feature=exemplar-storage`.
The Perses Prometheus datasource has `exemplars.enable: true`, so any time
series panel draws exemplars when its series have them. Perses' heatmap and
histogram panels only read native histograms, which is why these use the
exponential span metrics rather than OBI's classic-bucket ones.

Perses re-reads the folder every 30 seconds, so you can edit a dashboard file
and refresh. Perses validates every panel against its plugin schemas on load.

## Repo layout

```
services/
  gateway/    Go 1.27, net/http + grpc-go   (port 8088, published as 8080)
  catalog/    Python 3.14, FastAPI + psycopg (port 8000)
  cart/       Node.js 24, node:http + redis  (port 3000)
  checkout/   Java 25, grpc-java + jnats     (port 50051)
  notifier/   Rust, async-nats + reqwest     (port 7070, health only)
proto/        checkout.proto (gRPC contract; Go code is generated into the gateway)
config/
  obi/        OBI v1 config (Config v2 schema)
  otelcol/    Collector config
  prometheus/ Prometheus config
  perses/     Perses server config and provisioned resources
  postgres/   Product seed data
loadgen/      k6 script
lima/         Lima VM definition
deck/         Companion talk for demoing this repo live (decker; `cd deck && go run .`)
```

## Pinned versions

| Component | Version |
| --- | --- |
| OBI | `otel/ebpf-instrument:v1.0.0` |
| OTel Collector contrib | 0.165.0 |
| Jaeger | 2.20.0 (see below) |
| Prometheus | v3.16.0 |
| Perses | v0.55.1 |
| k6 | 2.3.0 |
| Postgres / Redis / NATS | 18 / 8.8 / 2.15 |

Jaeger is pinned to 2.20.0 because it's the newest release that still serves
the `/api/traces` endpoints Perses' Jaeger plugin (0.2.x) queries. Jaeger
2.21 and later only serve `/api/v3`. If you don't need the trace table in
Perses, any newer Jaeger v2 works for the rest of the demo.

## Troubleshooting

- **No data at all.** Run `make obi-logs`. OBI logs each process it
  instruments. `make check-kernel` shows the kernel, BTF and tracefs it sees.
- **A Go service isn't instrumented.** OBI's Go probes find functions by symbol
  name, so don't strip Go binaries (`-ldflags "-s -w"`). The gateway's
  Dockerfile keeps symbols.
- **Odd service names.** Each service sets `OTEL_SERVICE_NAME`; without it OBI
  falls back to the executable name (`python3.14`, `node`, `java`...).
- **The notifier has no NATS spans, only its HTTP calls to the catalog.** OBI
  v1 parses NATS out of its generic TCP tracker, which pairs something the
  process sends with the reply that comes back. A push subscriber never sends
  anything before a `MSG` arrives, so deliveries are never emitted. Only
  consumers that publish and receive on the same connection (request/reply)
  get `process` spans.
- **NATS publishes start their own traces.** jnats writes to the socket from a
  dedicated writer thread, not the gRPC handler thread, so OBI can't parent the
  `publish orders.placed` span under `PlaceOrder`.
- **The notifier's catalog calls are extra roots in the order's trace.** OBI v1
  can't carry trace context through the NATS broker, so the services forward it
  by hand: checkout copies the incoming `traceparent` into a NATS message
  header, and the notifier sends it on to the catalog with a new span ID. OBI
  adopts a `traceparent` it finds on an outgoing request instead of injecting
  its own, so the notifier and catalog spans join the order's trace. The
  notifier has no consumer span to hang them under, so they show up as extra
  root spans next to the gateway's.
- **NATS publish spans need the flush.** Core NATS `PUB` gets no reply, so
  without the `flush()` after each publish in checkout, OBI folds every publish
  into one open request until a keepalive `PONG` closes it, and records about
  one publish every few minutes. The `network.capture.buffer_size` setting in
  `config/obi/obi.yaml` matters too: at the default, most order messages are
  truncated and dropped.
- **Header capture rules match nothing.** In a Config v2 file, write header
  `patterns` in lowercase (`content-type`, not `Content-Type`). OBI v1.0.0
  lowercases the header name for `case_sensitive: false` but not the pattern,
  so mixed-case patterns never match. That includes `obfuscate` rules.
- **The service graph shows `incoming` or `outgoing`.** Those are OBI's names
  for peers it couldn't identify. OBI only sees the peer's IP, and outside
  Kubernetes it names it with a reverse lookup, which the `extra_hosts` on the
  `obi` service answers. So the shop's services have fixed addresses in
  `docker-compose.yaml`. If you add a service, give it an `ipv4_address` and an
  `extra_hosts` entry. Browser traffic comes in through the published port from
  the Docker bridge, so it stays `incoming`.
- **Telemetry is wrong for about a minute after restarting OBI.** OBI v1 can't
  fully read connections that were already open when it started. It doesn't
  trace Postgres on them at all. On gRPC it can't decode the HTTP/2 headers,
  which are compressed against a table built up since the connection opened,
  so `rpc.method` is `_OTHER` and the span is just `grpc`. Redis is unaffected.
  The shop recycles both kinds of connection every minute: the catalog's pool
  (`max_lifetime` in `services/catalog/app.py`) and checkout's gRPC server
  (`maxConnectionAge` in `CheckoutServer.java`). So the data recovers within
  about a minute.
- **Runtime metrics are missing.** They're experimental in v1. Go runtime
  metrics only appear after the first GC cycle.

## License

Apache-2.0. See [LICENSE](LICENSE).
