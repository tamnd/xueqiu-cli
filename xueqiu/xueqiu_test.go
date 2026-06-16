package xueqiu_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/xueqiu-cli/xueqiu"
)

// postDataJSON returns the inner JSON string for a timeline item.
func postDataJSON(id int64, title, desc, author string, replies, likes int, target string) string {
	b, _ := json.Marshal(map[string]any{
		"id":          id,
		"title":       title,
		"description": desc,
		"reply_count": replies,
		"like_count":  likes,
		"user":        map[string]any{"screen_name": author},
		"target":      target,
	})
	return string(b)
}

func newTestClient(t *testing.T, mux *http.ServeMux) *xueqiu.Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := xueqiu.DefaultConfig()
	cfg.Rate = 0
	cfg.Retries = 0
	cfg.Timeout = 5 * time.Second
	c := xueqiu.NewClientFromConfig(cfg)
	c.SetBaseURLs(srv.URL, srv.URL)
	c.SetHasSession(true) // skip bootstrap in tests
	return c
}

func TestHotPosts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v4/statuses/public_timeline_by_category.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		item1 := postDataJSON(394631267, "A股最大的认知误区", "本轮AI行情走到年末", "价值投资者", 5, 12, "/8778887955/394631267")
		item2 := postDataJSON(394770138, "白酒渠道变革", "白酒历史回顾", "白酒研究员", 3, 8, "/8778887955/394770138")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"list": []map[string]any{
				{"id": 394631267, "category": 0, "data": item1},
				{"id": 394770138, "category": 0, "data": item2},
			},
			"next_max_id": 394628000,
			"next_id":     394631267,
		})
	})

	c := newTestClient(t, mux)
	posts, err := c.HotPosts(context.Background(), 10)
	if err != nil {
		t.Fatalf("HotPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("want 2 posts, got %d", len(posts))
	}
	p := posts[0]
	if p.ID != 394631267 {
		t.Errorf("want id 394631267, got %d", p.ID)
	}
	if p.Title != "A股最大的认知误区" {
		t.Errorf("want title, got %q", p.Title)
	}
	if p.Author != "价值投资者" {
		t.Errorf("want author, got %q", p.Author)
	}
	if p.ReplyCount != 5 {
		t.Errorf("want reply_count 5, got %d", p.ReplyCount)
	}
	if p.LikeCount != 12 {
		t.Errorf("want like_count 12, got %d", p.LikeCount)
	}
}

func TestHotPostsLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v4/statuses/public_timeline_by_category.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := make([]map[string]any, 5)
		for i := range items {
			data := postDataJSON(int64(i+1), "Post", "Desc", "Author", 0, 0, "")
			items[i] = map[string]any{"id": i + 1, "category": 0, "data": data}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"list": items})
	})

	c := newTestClient(t, mux)
	posts, err := c.HotPosts(context.Background(), 3)
	if err != nil {
		t.Fatalf("HotPosts: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("want 3 posts after limit, got %d", len(posts))
	}
}

func TestHotPostsEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v4/statuses/public_timeline_by_category.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"list": []any{}})
	})

	c := newTestClient(t, mux)
	posts, err := c.HotPosts(context.Background(), 10)
	if err != nil {
		t.Fatalf("HotPosts: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("want 0 posts, got %d", len(posts))
	}
}

func TestHotPostsServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v4/statuses/public_timeline_by_category.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	})

	c := newTestClient(t, mux)
	_, err := c.HotPosts(context.Background(), 10)
	if err == nil {
		t.Fatal("want error on 500, got nil")
	}
}

func TestStockQuote(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/query/v1/suggest_stock.json", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != "SH000001" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    200,
			"message": "success",
			"success": true,
			"data": []map[string]any{
				{
					"code":    "SH000001",
					"name":    "上证指数",
					"current": 3301.36,
					"percent": -0.42,
					"chg":     -14.02,
					"open":    3315.38,
					"high":    3322.10,
					"low":     3296.50,
					"volume":  int64(1234567890),
					"amount":  float64(150000000000),
				},
			},
		})
	})

	c := newTestClient(t, mux)
	q, err := c.StockQuote(context.Background(), "SH000001")
	if err != nil {
		t.Fatalf("StockQuote: %v", err)
	}
	if q.Symbol != "SH000001" {
		t.Errorf("want symbol SH000001, got %q", q.Symbol)
	}
	if q.Name != "上证指数" {
		t.Errorf("want name 上证指数, got %q", q.Name)
	}
	if q.Current != 3301.36 {
		t.Errorf("want current 3301.36, got %f", q.Current)
	}
	if q.Percent != -0.42 {
		t.Errorf("want percent -0.42, got %f", q.Percent)
	}
}

func TestStockQuoteNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/query/v1/suggest_stock.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    200,
			"data":    []any{},
			"success": true,
		})
	})

	c := newTestClient(t, mux)
	_, err := c.StockQuote(context.Background(), "INVALID")
	if err == nil {
		t.Fatal("want error for not found symbol, got nil")
	}
}

func TestStockQuoteContextCancelled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/query/v1/suggest_stock.json", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	c := newTestClient(t, mux)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.StockQuote(ctx, "SH000001")
	if err == nil {
		t.Fatal("want error on cancelled context, got nil")
	}
}
