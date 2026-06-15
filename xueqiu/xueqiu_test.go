package xueqiu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient returns a Client wired to the given test server for both
// the prime URL and the stock API base.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient()
	c.Rate = 0 // no pacing in tests
	c.Retries = 0
	c.BaseURL = srv.URL
	c.StockBase = srv.URL
	return c
}

func TestHotStocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "hot_stock"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"data": {
						"items": [
							{"code":"SH600519","name":"贵州茅台","percent":1.23,"current":1800.00},
							{"code":"SZ000858","name":"五粮液","percent":-0.50,"current":150.00}
						]
					}
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.HotStocks(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Code != "SH600519" {
		t.Errorf("items[0].Code = %q, want SH600519", items[0].Code)
	}
	if items[0].Name != "贵州茅台" {
		t.Errorf("items[0].Name = %q, want 贵州茅台", items[0].Name)
	}
	if items[0].Current != 1800.00 {
		t.Errorf("items[0].Current = %v, want 1800.00", items[0].Current)
	}
	if items[0].Percent != 1.23 {
		t.Errorf("items[0].Percent = %v, want 1.23", items[0].Percent)
	}
}

func TestGetQuote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "quote.json"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"data": {
						"quote": {
							"symbol": "SH600519",
							"name": "贵州茅台",
							"current": 1800.00,
							"percent": 1.23,
							"chg": 21.84,
							"open": 1790.00,
							"high": 1820.00,
							"low": 1785.00,
							"volume": 3000000,
							"amount": 5400000000,
							"market_capital": 2268000000000,
							"currency": "CNY"
						}
					}
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	q, err := c.GetQuote(context.Background(), "SH600519")
	if err != nil {
		t.Fatal(err)
	}
	if q.Symbol != "SH600519" {
		t.Errorf("Symbol = %q, want SH600519", q.Symbol)
	}
	if q.Name != "贵州茅台" {
		t.Errorf("Name = %q, want 贵州茅台", q.Name)
	}
	if q.Current != 1800.00 {
		t.Errorf("Current = %v, want 1800.00", q.Current)
	}
	if q.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", q.Currency)
	}
	if q.Volume != 3000000 {
		t.Errorf("Volume = %d, want 3000000", q.Volume)
	}
}

func TestPrimeCachedOnSecondCall(t *testing.T) {
	var primeHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			primeHits++
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "hot_stock") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"data":{"items":[]}}}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _ = c.HotStocks(context.Background(), 5)
	_, _ = c.HotStocks(context.Background(), 5)

	if primeHits != 1 {
		t.Errorf("prime was called %d times, want 1", primeHits)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	body, err := c.get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
