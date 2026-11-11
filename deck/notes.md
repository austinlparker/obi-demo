# Help me, OBI wan Telemetry, you're my only hope!

A 10 minute talk, demo included, that introduces a general audience to
OpenTelemetry and OBI. The running joke is the hologram message: a droid
projects the title, and sits in the corner of every slide after it.

## A long time ago

Cold open, no talking. Build 2 starts the crawl; it takes about ten seconds
and stops with the whole text on screen. Advance when the room has read it.

## What Is OBI?

OBI stands for OpenTelemetry eBPF instrumentation, and its 1.0 release was this year.
It's a way to generate metrics + traces + logs from applications with no code changes needed.

## One click, five services

Let's start by explaining the problem we're solving generally. You go to a website and add something
to your cart, then click buy. That request passes through a bunch of services. Those services are written
in different languages, maintained by different teams. Sometimes you can't change the service at all.

Build 2: if you're a developer or an SRE, you want to know not just that the request was slow, but why.
OpenTelemetry is meant to solve this problem.

## The demo: a swag shop

Here is that system for real. It's a small shop, it's running on this laptop,
and it's what the demo will show.

Build 1: a gateway serves the UI and the API. A load generator sends about 8
shopping sessions a second.

Build 2: the gateway calls the cart, which uses Redis, and the catalog, which
uses Postgres, over HTTP.

Build 3: checkout is a gRPC service. It declines about 3% of cards.

Build 4: checkout publishes to NATS; the notifier consumes and tells the
catalog how much stock was sold.

Build 5: the same picture by language: Go, Node.js, Python, Java, Rust. Five
services, five languages. Keep that in mind for the next few slides.

## OpenTelemetry

OpenTelemetry is the open standard for describing what software is doing. It
is a CNCF project and no vendor owns it.

Build 2: three kinds of data. Traces follow one request, metrics count and
measure, logs record events.

Build 3: all of it travels in one format, OTLP, so it can go to any backend,
open source or commercial. Instrument once and the choice of backend stays
open.

## One request, one trace

Let's look at what our earlier request would look like if it was a trace.

Build 2: each bar below is a span, one piece of work with a start and an end:
the call to the cart, the call to the catalog, the catalog's database query,
the call to checkout. They share one trace ID, which is how they are joined.

Build 3: with the trace on screen the earlier question answers itself. The
catalog took most of the time, and its database query didn't.

## The usual way: an SDK

How does a service produce spans? Normally with the OpenTelemetry SDK, a
library that runs inside the process, next to your code.

Build 2: the SDK wraps your handlers and clients, and exports over OTLP. To
get it there you add a dependency or an agent, configure it and redeploy.

Build 3: and you do that once per service, with a different SDK for each
language. Five languages means five ways to do it.

Build 4, the question mark: then there is everything you can't change: the legacy service nobody
wants to touch, the vendor binary, the team with no time this quarter. Those
are the gaps in the trace.

## eBPF: code in the kernel

Every program, in any language, asks the Linux kernel to do its network and
disk I/O. The kernel is the one place all of it passes through.

Build 2: eBPF lets you load small programs into the kernel that run when
something happens: a packet is sent, a function is called. They can watch
without the application knowing.

## How OBI sees a request

The gateway calls the catalog. The request leaves the gateway, goes through
the kernel and arrives at the catalog.

Build 2: OBI attaches eBPF probes on that path, where data is sent and where
it is received. For some runtimes it also attaches to functions inside the
process: Go's HTTP and gRPC libraries, for example.

Build 3: the probes record what was said and how long it took. OBI, a normal
process in user space, parses the protocol: HTTP, gRPC, SQL, Redis, NATS,
Kafka and more.

Build 4: it turns that into the same spans and metrics an SDK would produce,
and exports them over OTLP. Nothing in the gateway or the catalog changed, and
neither one imports OpenTelemetry.

Because one tool produced all of it, the span names and metric names are the
same for every service, whatever it's written in. And it is OpenTelemetry:
standard data, OTLP out, any backend.

## Joining the hops

Seeing each hop is not enough. They have to join into one trace, and that
needs the trace ID to travel with the request. The standard way is a header
called `traceparent`.

Build 2: an SDK would add that header in your code. OBI adds it as the request
leaves, with `context_propagation: all`.

On the other side OBI reads it as the request arrives, so both spans
land in the same trace. If the application already sets `traceparent`, OBI
uses that one.

Where it stops: OBI joins a request to its reply on one connection. A message
broker is two separate connections with a queue in between, and OBI doesn't
carry the trace across it. The demo has one of those, and we'll look at what
that costs after the demo.

For the experts: header injection covers HTTP/1 and plaintext HTTP/2 and gRPC.
For Go, OBI writes the header inside the process, through the library probes
from the last slide. A proxy that terminates connections between two services
breaks the TCP-level variant. The docs list the limits.

## The whole setup

This is the instrumentation work for five services. The slide is abridged: the
real file has one include rule per service, plus the exporters. OBI runs as
one container that shares the host's process and network namespaces, so it
sees all five.

Build 2: exclude everything by default, so the databases and the telemetry
stack are left alone.

Build 3: include a service by the port it listens on. Five services, five
rules.

Build 4: one line turns on context propagation.

## Demo

Leave this slide up and switch windows.

1. Shop on :8080, then Jaeger on :16686. Open a `POST /api/checkout` trace:
   Go, Java, Python, Node.js and Rust spans in one trace, with the SQL and
   Redis calls underneath. Search Jaeger for a tag like
   `http.response.header.x-order-id` to find one order: OBI captured that
   header off the wire.
2. Perses on :8090, project "Swag shop": request rate, errors and duration per
   service, and the service graph. Nothing was configured per service. Card
   declines show up as the gateway's 402s, not as checkout errors.
3. `make chaos-latency`: the catalog gets about 400 ms slower. Open the
   "latency histograms and exemplars" dashboard and pick the catalog: the
   heatmap shifts up. On the p95 panel, hover an exemplar diamond for its
   `trace_id`, paste it into Jaeger's trace ID search and land on a slow
   trace. `make chaos-errors` fails 20% of cart requests. `make chaos-off`
   when done.

Where the telemetry goes, if asked: OBI sends OTLP to the OpenTelemetry
Collector, which sends traces to Jaeger and metrics to Prometheus. The
Collector also builds latency histograms from OBI's spans with its spanmetrics
connector and attaches exemplars; OBI's own metrics don't carry them. Perses
reads both, and its dashboards are YAML in the repo.

About NATS, if asked: the notifier's calls are in the checkout trace because
checkout and the notifier copy `traceparent` through the message by hand, a
few lines each and no OpenTelemetry library. The publish shows up as a short
trace of its own, and there is no span for the consume.

If asked about the laptop: this runs in one Lima VM. OBI must share a kernel
with what it watches, and Apple's `container` tool gives every container its
own kernel.

Don't restart OBI just before going on: Postgres and gRPC spans are wrong for
about a minute afterwards.

## Breadth and depth

Back from the demo. Be straight about the trade. OBI sees what crosses the
wire: every service, every call, with no code. That includes headers and
bodies if you ask for them, which is how the order ID got into that trace.

Build 2: it can't see inside your process. An SDK knows the customer ID, the
cart value, the feature flag, the step inside one function. And it can't
follow a request through a queue: in the demo, the hop through NATS joins the
trace only because two services pass the trace header along by hand. OBI's
docs say the same: use language instrumentation for custom spans and business
attributes.

So use both: start wide, go deep where it matters. OBI gives the whole map on
day one, with no pull requests to every team, and it covers the services you
can't change. Add SDKs to the few services where you need depth. The two join
into the same traces.

Other honest limits: Linux only, kernel 5.8 or newer with BTF, and it needs
elevated privileges.

## Where OBI runs

One rule: OBI has to share a kernel with what it watches. Everything else
follows from that.

Build 1: on a Linux host it is one process, a single binary, and it sees the
other processes on that host.

Build 2: with Docker it is one container that shares the host's process
namespace. That is how the demo runs.

Build 3: on Kubernetes, pods on a node share that node's kernel, so OBI runs
as a DaemonSet: one pod per node, with `hostPID`. A sidecar per pod is also
supported, and there is a Helm chart. It adds Kubernetes metadata to
everything it reports: namespace, deployment, pod, node.

Selecting what to instrument is the same config you saw earlier in all three.
OBI can also run inside the OpenTelemetry Collector, as a receiver.

## Try it

The repo has everything shown here. On a Mac it needs Lima; on Linux it runs
on Docker directly. Two commands and a few minutes for the first build.

The OBI docs are under zero-code instrumentation on opentelemetry.io.
