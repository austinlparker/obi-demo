"""Catalog service: products and stock, stored in Postgres.

No OpenTelemetry SDK here. OBI sees the HTTP server spans (with FastAPI
routes) and the PostgreSQL client calls from outside the process.
"""

import os
import random
import time
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from psycopg_pool import ConnectionPool
from pydantic import BaseModel

pool = ConnectionPool(
    os.environ.get("DATABASE_URL", "postgresql://shop:shop@postgres:5432/shop"),
    min_size=1,
    max_size=10,
    open=False,
)

# Fault injection, changed at runtime through POST /admin/chaos.
chaos = {"error_rate": 0.0, "latency_ms": 0}



@asynccontextmanager
async def lifespan(_: FastAPI):
    pool.open(wait=True, timeout=60)
    yield
    pool.close()


app = FastAPI(title="catalog", lifespan=lifespan)


def maybe_misbehave() -> None:
    if chaos["latency_ms"]:
        # Jitter so latency histograms look like real latency.
        time.sleep(chaos["latency_ms"] * random.uniform(0.5, 1.5) / 1000)
    if random.random() < chaos["error_rate"]:
        raise HTTPException(status_code=500, detail="injected catalog failure")


class Sold(BaseModel):
    quantity: int


class Chaos(BaseModel):
    error_rate: float = 0.0
    latency_ms: int = 0


def row_to_product(row) -> dict:
    sku, name, price_cents, stock = row
    return {"sku": sku, "name": name, "price_cents": price_cents, "stock": stock}


@app.get("/healthz")
def healthz() -> str:
    return "ok"


@app.get("/products")
def list_products() -> list[dict]:
    maybe_misbehave()
    with pool.connection() as conn:
        rows = conn.execute("SELECT sku, name, price_cents, stock FROM products ORDER BY sku").fetchall()
    return [row_to_product(r) for r in rows]


@app.get("/products/{sku}")
def get_product(sku: str) -> dict:
    maybe_misbehave()
    with pool.connection() as conn:
        row = conn.execute(
            "SELECT sku, name, price_cents, stock FROM products WHERE sku = %s", (sku,)
        ).fetchone()
    if row is None:
        raise HTTPException(status_code=404, detail=f"no product {sku}")
    return row_to_product(row)


@app.post("/products/{sku}/sold")
def product_sold(sku: str, sold: Sold) -> dict:
    with pool.connection() as conn:
        # Restock instead of running out, so the demo can run all day.
        row = conn.execute(
            """
            UPDATE products
               SET stock = CASE WHEN stock - %s < 0 THEN 100 ELSE stock - %s END
             WHERE sku = %s
         RETURNING sku, name, price_cents, stock
            """,
            (sold.quantity, sold.quantity, sku),
        ).fetchone()
    if row is None:
        raise HTTPException(status_code=404, detail=f"no product {sku}")
    return row_to_product(row)


@app.post("/admin/chaos")
def set_chaos(setting: Chaos) -> dict:
    chaos.update(setting.model_dump())
    return {"service": "catalog", **chaos}
