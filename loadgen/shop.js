// Steady shopping traffic for the swag shop. Runs in the k6 container.
// RATE (iterations per second) and DURATION can be overridden with env vars.
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.GATEWAY_URL || 'http://gateway:8088';
const SKUS = ['otel-hoodie', 'obi-sticker', 'ebpf-mug', 'collector-cap', 'trace-socks', 'kubecon-tote'];

export const options = {
  scenarios: {
    shoppers: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 8),
      timeUnit: '1s',
      duration: __ENV.DURATION || '24h',
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
  // The demo injects failures on purpose; don't let k6 treat them as fatal.
  thresholds: {},
  summaryTrendStats: ['avg', 'p(95)'],
};

const json = { headers: { 'Content-Type': 'application/json' } };
const pick = (a) => a[Math.floor(Math.random() * a.length)];

export default function () {
  const user = `user-${Math.floor(Math.random() * 50)}`;

  const products = http.get(`${BASE}/api/products`, { tags: { name: 'list products' } });
  check(products, { 'products 200': (r) => r.status === 200 });

  const sku = pick(SKUS);
  http.get(`${BASE}/api/products/${sku}`, { tags: { name: 'get product' } });

  const adds = 1 + Math.floor(Math.random() * 3);
  for (let i = 0; i < adds; i++) {
    const res = http.post(
      `${BASE}/api/cart/${user}/items`,
      JSON.stringify({ sku: pick(SKUS), quantity: 1 + Math.floor(Math.random() * 2) }),
      Object.assign({ tags: { name: 'add to cart' } }, json),
    );
    check(res, { 'add 200': (r) => r.status === 200 });
  }

  http.get(`${BASE}/api/cart/${user}`, { tags: { name: 'view cart' } });

  if (Math.random() < 0.35) {
    const res = http.post(`${BASE}/api/checkout/${user}`, null, { tags: { name: 'checkout' } });
    check(res, { 'checkout ok or declined': (r) => r.status === 200 || r.status === 402 });
  }

  sleep(Math.random());
}
