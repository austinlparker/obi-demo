package io.swagshop.checkout;

import io.grpc.Server;
import io.grpc.Status;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.stub.StreamObserver;
import io.nats.client.Connection;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.swagshop.checkout.v1.CheckoutServiceGrpc;
import io.swagshop.checkout.v1.LineItem;
import io.swagshop.checkout.v1.PlaceOrderRequest;
import io.swagshop.checkout.v1.PlaceOrderResponse;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.ThreadLocalRandom;
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
        .addService(new CheckoutService(nats, subject, declineRate))
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
      nats.publish(subject, event.getBytes(StandardCharsets.UTF_8));

      out.onNext(PlaceOrderResponse.newBuilder().setOrderId(orderId).setTotalCents(total).build());
      out.onCompleted();
    }
  }
}
