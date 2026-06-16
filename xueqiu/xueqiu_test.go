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

func TestKline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "kline"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"symbol": "SH600519",
					"column": ["timestamp","volume","open","high","low","close","chg","percent","turnoverrate","amount"],
					"item": [
						[1700000000000, 1000000, 1780.0, 1820.0, 1770.0, 1800.0, 20.0, 1.12, 0.05, 1800000000],
						[1700086400000, 1200000, 1800.0, 1850.0, 1790.0, 1840.0, 40.0, 2.22, 0.06, 2208000000]
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	candles, err := c.Kline(context.Background(), "SH600519", "day", -100)
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("got %d candles, want 2", len(candles))
	}
	if candles[0].Symbol != "SH600519" {
		t.Errorf("Symbol = %q, want SH600519", candles[0].Symbol)
	}
	if candles[0].Open != 1780.0 {
		t.Errorf("Open = %v, want 1780.0", candles[0].Open)
	}
	if candles[0].High != 1820.0 {
		t.Errorf("High = %v, want 1820.0", candles[0].High)
	}
	if candles[0].Low != 1770.0 {
		t.Errorf("Low = %v, want 1770.0", candles[0].Low)
	}
	if candles[0].Close != 1800.0 {
		t.Errorf("Close = %v, want 1800.0", candles[0].Close)
	}
	if candles[0].Volume != 1000000 {
		t.Errorf("Volume = %d, want 1000000", candles[0].Volume)
	}
	if candles[0].Timestamp != 1700000000000 {
		t.Errorf("Timestamp = %d, want 1700000000000", candles[0].Timestamp)
	}
}

func TestStocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "screener/quote/list"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"list": [
						{"symbol":"SH600519","name":"贵州茅台","current":1800.0,"percent":1.23,"volume":3000000,"amount":5400000000},
						{"symbol":"SZ000858","name":"五粮液","current":150.0,"percent":-0.5,"volume":5000000,"amount":750000000}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	stocks, err := c.Stocks(context.Background(), "CN", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks) != 2 {
		t.Fatalf("got %d stocks, want 2", len(stocks))
	}
	if stocks[0].Symbol != "SH600519" {
		t.Errorf("Symbol = %q, want SH600519", stocks[0].Symbol)
	}
	if stocks[0].Current != 1800.0 {
		t.Errorf("Current = %v, want 1800.0", stocks[0].Current)
	}
}

func TestScreener(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "screener/quote/list"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"list": [
						{"symbol":"SH600519","name":"贵州茅台","current":1800.0,"percent":1.23,"volume":3000000,"amount":5400000000,"pe_ttm":35.6}
					]
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	stocks, err := c.Screener(context.Background(), "CN", 10, 50, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks) != 1 {
		t.Fatalf("got %d stocks, want 1", len(stocks))
	}
	if stocks[0].PeTTM != 35.6 {
		t.Errorf("PeTTM = %v, want 35.6", stocks[0].PeTTM)
	}
}

func TestNews(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "stock_timeline"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"statuses": [
					{"id":1001,"title":"贵州茅台年报分析","description":"业绩稳健增长...","created_at":1700000000000,"reply_count":42,"fav_count":88},
					{"id":1002,"title":"茅台估值探讨","description":"当前PE是否合理...","created_at":1699900000000,"reply_count":15,"fav_count":30}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.News(context.Background(), "SH600519", 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].ID != 1001 {
		t.Errorf("ID = %d, want 1001", items[0].ID)
	}
	if items[0].Title != "贵州茅台年报分析" {
		t.Errorf("Title = %q, want 贵州茅台年报分析", items[0].Title)
	}
	if items[0].ReplyCount != 42 {
		t.Errorf("ReplyCount = %d, want 42", items[0].ReplyCount)
	}
}

func TestComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "query.json"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"statuses": [
					{
						"id": 2001,
						"title": "看好茅台长期价值",
						"description": "消费升级大趋势...",
						"created_at": 1700000000000,
						"reply_count": 10,
						"fav_count": 50,
						"user": {"id": 9001, "screen_name": "投资者A", "followers_count": 1234}
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv)
	items, err := c.Comments(context.Background(), "SH600519", 15, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].ID != 2001 {
		t.Errorf("ID = %d, want 2001", items[0].ID)
	}
	if items[0].UserName != "投资者A" {
		t.Errorf("UserName = %q, want 投资者A", items[0].UserName)
	}
	if items[0].FollowersCount != 1234 {
		t.Errorf("FollowersCount = %d, want 1234", items[0].FollowersCount)
	}
	if items[0].UserID != 9001 {
		t.Errorf("UserID = %d, want 9001", items[0].UserID)
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
