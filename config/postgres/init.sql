CREATE TABLE products (
  sku         TEXT PRIMARY KEY,
  name        TEXT    NOT NULL,
  price_cents BIGINT  NOT NULL,
  stock       INTEGER NOT NULL
);

INSERT INTO products (sku, name, price_cents, stock) VALUES
  ('otel-hoodie',   'OpenTelemetry hoodie',          5500, 100),
  ('obi-sticker',   'OBI bee sticker pack',            500, 100),
  ('ebpf-mug',      'eBPF "it''s just a probe" mug',  1800, 100),
  ('collector-cap', 'Collector dad cap',              2500, 100),
  ('trace-socks',   'Distributed trace socks',        1200, 100),
  ('kubecon-tote',  'KubeCon NA 2026 tote',           1500, 100);
