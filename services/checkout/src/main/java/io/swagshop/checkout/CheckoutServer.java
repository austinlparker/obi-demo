package io.swagshop.checkout;

import io.grpc.Context;
import io.grpc.Contexts;
import io.grpc.Metadata;
import io.grpc.Server;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.ServerInterceptors;
import io.grpc.Status;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.StreamObserver;
import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.impl.Headers;
import io.swagshop.checkout.v1.CheckoutServiceGrpc;
import io.swagshop.checkout.v1.LineItem;
import io.swagshop.checkout.v1.PlaceOrderRequest;
import io.swagshop.checkout.v1.PlaceOrderResponse;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * Checkout over gRPC. "Charges" the order, then publishes it to NATS for the
 * notifier. There is no OpenTelemetry agent on the JVM: OBI reads the gRPC
 * (HTTP/2) and NATS traffic from the kernel.
 */
public final class CheckoutServer {
  private static final Logger LOG = Logger.getLogger(CheckoutServer.class.getName());

  private static String env(String key, String def) {
    String v = System.getenv(key);
    return v == null || v.isBlank() ? def : v;
  }

  public static void main(String[] args) throws Exception {
    int port = Integer.parseInt(env("PORT", "50051"));
    double declineRate = Double.parseDouble(env("CHECKOUT_DECLINE_RATE", "0.03"));
    String subject = env("ORDERS_SUBJECT", "orders.placed");

    Options opts = new Options.Builder()
        .server(env("NATS_URL", "nats://nats:4222"))
        .connectionName("checkout")
        .maxReconnects(-1)
        .reconnectWait(Duration.ofSeconds(1))
        .build();
    Connection nats = connectWithRetry(opts);

    Server server = NettyServerBuilder.forPort(port)
        // OBI reads gRPC from the wire, and HTTP/2 compresses headers against
        // a table built up over the connection's life. After an OBI restart it
        // can't decode connections that were already open, so rpc.method turns
        // into _OTHER. Capping connection age makes the gateway reconnect
        // within a minute. grpc-go does that transparently on GOAWAY.
        .maxConnectionAge(60, TimeUnit.SECONDS)
        .maxConnectionAgeGrace(10, TimeUnit.SECONDS)
        .addService(ServerInterceptors.intercept(
            new CheckoutService(nats, subject, declineRate), new TraceparentInterceptor()))
        .build()
        .start();
    LOG.info("checkout listening on :" + port + ", decline rate " + declineRate);
    Runtime.getRuntime().addShutdownHook(new Thread(server::shutdown));
    server.awaitTermination();
  }

  private static Connection connectWithRetry(Options opts) throws InterruptedException {
    while (true) {
      try {
        return Nats.connect(opts);
      } catch (Exception e) {
        LOG.warning("waiting for NATS: " + e.getMessage());
        Thread.sleep(1000);
      }
    }
  }

  static final Context.Key<String> TRACEPARENT = Context.key("traceparent");
  private static final Metadata.Key<String> TRACEPARENT_HEADER =
      Metadata.Key.of("traceparent", Metadata.ASCII_STRING_MARSHALLER);

  /**
   * OBI puts a W3C traceparent on the gateway's gRPC call, but it can't carry it
   * through NATS: the broker sits between two separate connections. So the
   * handler forwards it by hand, as a NATS message header. This is plain header
   * plumbing, no OpenTelemetry library.
   */
  static final class TraceparentInterceptor implements ServerInterceptor {
    @Override
    public <Q, R> ServerCall.Listener<Q> interceptCall(
        ServerCall<Q, R> call, Metadata headers, ServerCallHandler<Q, R> next) {
      String tp = headers.get(TRACEPARENT_HEADER);
      if (tp == null) {
        return next.startCall(call, headers);
      }
      return Contexts.interceptCall(Context.current().withValue(TRACEPARENT, tp), call, headers, next);
    }
  }

  static final class CheckoutService extends CheckoutServiceGrpc.CheckoutServiceImplBase {
    private final Connection nats;
    private final String subject;
    private final double declineRate;

    CheckoutService(Connection nats, String subject, double declineRate) {
      this.nats = nats;
      this.subject = subject;
      this.declineRate = declineRate;
    }

    @Override
    public void placeOrder(PlaceOrderRequest req, StreamObserver<PlaceOrderResponse> out) {
      ThreadLocalRandom rnd = ThreadLocalRandom.current();

      // Pretend to talk to a payment provider.
      try {
        Thread.sleep(rnd.nextLong(20, 120));
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
      }
      if (rnd.nextDouble() < declineRate) {
        out.onError(Status.FAILED_PRECONDITION.withDescription("card declined").asRuntimeException());
        return;
      }

      long total = 0;
      StringBuilder items = new StringBuilder();
      for (LineItem item : req.getItemsList()) {
        total += item.getUnitPriceCents() * item.getQuantity();
        if (items.length() > 0) {
          items.append(',');
        }
        items.append(String.format("{\"sku\":\"%s\",\"quantity\":%d}", item.getSku(), item.getQuantity()));
      }
      String orderId = "ord-" + UUID.randomUUID().toString().substring(0, 8);
      String event = String.format(
          "{\"order_id\":\"%s\",\"user_id\":\"%s\",\"total_cents\":%d,\"items\":[%s]}",
          orderId, req.getUserId(), total, items);
      Headers natsHeaders = new Headers();
      String tp = TRACEPARENT.get();
      if (tp != null) {
        natsHeaders.add("traceparent", tp);
      }
      nats.publish(subject, natsHeaders, event.getBytes(StandardCharsets.UTF_8));
      // Core NATS PUB has no reply, so OBI's request/response TCP tracker would
      // fold every publish into one open request until a keepalive PONG closes it.
      // The flush PING gets a PONG back, so each publish becomes its own span.
      try {
        nats.flush(Duration.ofSeconds(1));
      } catch (Exception e) {
        if (e instanceof InterruptedException) {
          Thread.currentThread().interrupt();
        }
        out.onError(Status.UNAVAILABLE.withDescription("order not handed to NATS").withCause(e).asRuntimeException());
        return;
      }

      out.onNext(PlaceOrderResponse.newBuilder().setOrderId(orderId).setTotalCents(total).build());
      out.onCompleted();
    }
  }
}
