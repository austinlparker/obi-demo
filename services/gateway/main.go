// Command gateway is the swag shop's public API. It fans out to the catalog
// (HTTP), the cart (HTTP) and checkout (gRPC). It contains no OpenTelemetry
// code: every span and metric you see for it comes from OBI.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	checkoutv1 "github.com/austinlparker/obi-demo/services/gateway/gen/checkoutv1"
)

//go:embed static
var staticFiles embed.FS

type product struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Stock      int    `json:"stock"`
}

type cartItem struct {
	SKU      string `json:"sku"`
	Quantity int32  `json:"quantity"`
}

type server struct {
	catalogURL string
	cartURL    string
	http       *http.Client
	checkout   checkoutv1.CheckoutServiceClient
	log        *slog.Logger
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	conn, err := grpc.NewClient(env("CHECKOUT_ADDR", "checkout:50051"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("dial checkout", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	s := &server{
		catalogURL: env("CATALOG_URL", "http://catalog:8000"),
		cartURL:    env("CART_URL", "http://cart:3000"),
		http:       &http.Client{Timeout: 5 * time.Second},
		checkout:   checkoutv1.NewCheckoutServiceClient(conn),
		log:        log,
	}

	static, _ := fs.Sub(staticFiles, "static")
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/products", s.listProducts)
	mux.HandleFunc("GET /api/products/{sku}", s.getProduct)
	mux.HandleFunc("GET /api/cart/{user}", s.getCart)
	mux.HandleFunc("POST /api/cart/{user}/items", s.addToCart)
	mux.HandleFunc("POST /api/checkout/{user}", s.placeOrder)
	mux.HandleFunc("POST /admin/chaos/{service}", s.chaos)

	addr := ":" + env("PORT", "8088")
	log.Info("gateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// call does an HTTP request to a backend and decodes a JSON response into out.
func (s *server) call(ctx context.Context, method, url string, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp.StatusCode, fmt.Errorf("%s %s: %d %s", method, url, resp.StatusCode, bytes.TrimSpace(msg))
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *server) fail(w http.ResponseWriter, code int, err error) {
	if code < 400 {
		code = http.StatusBadGateway
	}
	s.log.Warn("request failed", "code", code, "err", err)
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *server) listProducts(w http.ResponseWriter, r *http.Request) {
	var ps []product
	if code, err := s.call(r.Context(), http.MethodGet, s.catalogURL+"/products", nil, &ps); err != nil {
		s.fail(w, code, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (s *server) getProduct(w http.ResponseWriter, r *http.Request) {
	var p product
	if code, err := s.call(r.Context(), http.MethodGet, s.catalogURL+"/products/"+r.PathValue("sku"), nil, &p); err != nil {
		s.fail(w, code, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *server) getCart(w http.ResponseWriter, r *http.Request) {
	var items []cartItem
	if code, err := s.call(r.Context(), http.MethodGet, s.cartURL+"/cart/"+r.PathValue("user"), nil, &items); err != nil {
		s.fail(w, code, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *server) addToCart(w http.ResponseWriter, r *http.Request) {
	var item cartItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil || item.SKU == "" || item.Quantity <= 0 {
		s.fail(w, http.StatusBadRequest, errors.New("body must be {\"sku\": string, \"quantity\": int > 0}"))
		return
	}
	// Make sure the product exists before it goes in the cart.
	var p product
	if code, err := s.call(r.Context(), http.MethodGet, s.catalogURL+"/products/"+item.SKU, nil, &p); err != nil {
		s.fail(w, code, err)
		return
	}
	var items []cartItem
	if code, err := s.call(r.Context(), http.MethodPost, s.cartURL+"/cart/"+r.PathValue("user")+"/items", item, &items); err != nil {
		s.fail(w, code, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *server) placeOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := r.PathValue("user")

	var items []cartItem
	if code, err := s.call(ctx, http.MethodGet, s.cartURL+"/cart/"+user, nil, &items); err != nil {
		s.fail(w, code, err)
		return
	}
	if len(items) == 0 {
		s.fail(w, http.StatusBadRequest, errors.New("cart is empty"))
		return
	}

	req := &checkoutv1.PlaceOrderRequest{UserId: user}
	for _, it := range items {
		var p product
		if code, err := s.call(ctx, http.MethodGet, s.catalogURL+"/products/"+it.SKU, nil, &p); err != nil {
			s.fail(w, code, err)
			return
		}
		req.Items = append(req.Items, &checkoutv1.LineItem{Sku: it.SKU, Quantity: it.Quantity, UnitPriceCents: p.PriceCents})
	}

	resp, err := s.checkout.PlaceOrder(ctx, req)
	if err != nil {
		code := http.StatusBadGateway
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			code = http.StatusPaymentRequired
		}
		s.fail(w, code, err)
		return
	}

	if code, err := s.call(ctx, http.MethodDelete, s.cartURL+"/cart/"+user, nil, nil); err != nil {
		s.fail(w, code, err)
		return
	}
	// OBI records this header on the span (see config/obi/obi.yaml), which makes
	// the order ID a searchable span attribute without an SDK.
	w.Header().Set("X-Order-Id", resp.OrderId)
	writeJSON(w, http.StatusOK, map[string]any{"order_id": resp.OrderId, "total_cents": resp.TotalCents})
}

// chaos forwards a fault-injection setting to the catalog or the cart.
func (s *server) chaos(w http.ResponseWriter, r *http.Request) {
	var target string
	switch r.PathValue("service") {
	case "catalog":
		target = s.catalogURL
	case "cart":
		target = s.cartURL
	default:
		s.fail(w, http.StatusNotFound, errors.New("service must be catalog or cart"))
		return
	}
	var setting map[string]any
	if err := json.NewDecoder(r.Body).Decode(&setting); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	var out map[string]any
	if code, err := s.call(r.Context(), http.MethodPost, target+"/admin/chaos", setting, &out); err != nil {
		s.fail(w, code, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
