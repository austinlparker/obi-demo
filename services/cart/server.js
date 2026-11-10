// Cart service: a user's cart is a Redis hash of sku -> quantity.
// Plain node:http, no framework and no OpenTelemetry SDK. OBI captures the
// HTTP server spans and the Redis client commands from the kernel.
import http from 'node:http';
import { createClient } from 'redis';

const port = Number(process.env.PORT ?? 3000);
const redis = createClient({ url: process.env.REDIS_URL ?? 'redis://redis:6379' });
redis.on('error', (err) => console.error('redis error', err.message));
await redis.connect();

// Fault injection, changed at runtime through POST /admin/chaos.
const chaos = { error_rate: 0, latency_ms: 0 };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const key = (user) => `cart:${user}`;

async function readCart(user) {
  const h = await redis.hGetAll(key(user));
  return Object.entries(h).map(([sku, q]) => ({ sku, quantity: Number(q) }));
}

async function readBody(req) {
  let raw = '';
  for await (const chunk of req) raw += chunk;
  return raw ? JSON.parse(raw) : {};
}

function send(res, code, body) {
  const payload = JSON.stringify(body);
  res.writeHead(code, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(payload) });
  res.end(payload);
}

async function handle(req, res) {
  const url = new URL(req.url, 'http://cart');
  const parts = url.pathname.split('/').filter(Boolean);

  if (req.method === 'GET' && url.pathname === '/healthz') return send(res, 200, 'ok');

  if (req.method === 'POST' && url.pathname === '/admin/chaos') {
    const body = await readBody(req);
    chaos.error_rate = Number(body.error_rate ?? 0);
    chaos.latency_ms = Number(body.latency_ms ?? 0);
    return send(res, 200, { service: 'cart', ...chaos });
  }

  if (parts[0] !== 'cart' || !parts[1]) return send(res, 404, { error: 'not found' });

  if (chaos.latency_ms) await sleep(chaos.latency_ms * (0.5 + Math.random()));
  if (Math.random() < chaos.error_rate) return send(res, 500, { error: 'injected cart failure' });

  const user = parts[1];
  if (parts.length === 2 && req.method === 'GET') return send(res, 200, await readCart(user));
  if (parts.length === 2 && req.method === 'DELETE') {
    await redis.del(key(user));
    return send(res, 200, []);
  }
  if (parts.length === 3 && parts[2] === 'items' && req.method === 'POST') {
    const { sku, quantity } = await readBody(req);
    if (!sku || !(quantity > 0)) return send(res, 400, { error: 'sku and quantity > 0 required' });
    await redis.hIncrBy(key(user), sku, quantity);
    await redis.expire(key(user), 3600);
    return send(res, 200, await readCart(user));
  }
  return send(res, 405, { error: 'method not allowed' });
}

http
  .createServer((req, res) => {
    handle(req, res).catch((err) => {
      console.error(err);
      send(res, 500, { error: err.message });
    });
  })
  .listen(port, () => console.log(`cart listening on :${port}`));
