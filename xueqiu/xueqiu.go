// Package xueqiu is the library behind the xue command line:
// the HTTP client, session management, and typed data models for Xueqiu APIs.
//
// Xueqiu (xueqiu.com) requires a session cookie that is obtained automatically
// by visiting the site home page before making API calls. No account or login is
// required. The client paces requests, retries transient errors, and handles the
// double-JSON encoding Xueqiu uses for timeline items.
package xueqiu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to Xueqiu servers.
const DefaultUserAgent = "xue/dev (+https://github.com/tamnd/xueqiu-cli)"

// Host is the primary hostname the URI driver claims.
const Host = "xueqiu.com"

// BaseURL is the root URL for xueqiu.com.
const BaseURL = "https://" + Host

// ErrNotFound is returned when the API returns no results for a symbol.
var ErrNotFound = errors.New("not found")

// Config holds HTTP client parameters.
type Config struct {
	UserAgent  string
	Rate       time.Duration
	Retries    int
	Timeout    time.Duration
	XueqiuBase string // overridable for tests
	StockBase  string // overridable for tests
}

// DefaultConfig returns sensible defaults for Xueqiu APIs.
func DefaultConfig() Config {
	return Config{
		UserAgent:  DefaultUserAgent,
		Rate:       300 * time.Millisecond,
		Retries:    3,
		Timeout:    30 * time.Second,
		XueqiuBase: "https://xueqiu.com",
		StockBase:  "https://stock.xueqiu.com",
	}
}

// Client is a rate-limited HTTP client for Xueqiu public APIs.
type Client struct {
	cfg        Config
	http       *http.Client
	jar        http.CookieJar
	mu         sync.Mutex
	last       time.Time
	hasSession bool
}

// NewClient returns a Client with sensible defaults.
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	cfg := DefaultConfig()
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
			Jar:     jar,
		},
		jar: jar,
	}
}

// NewClientFromConfig returns a Client configured with cfg.
func NewClientFromConfig(cfg Config) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
			Jar:     jar,
		},
		jar: jar,
	}
}

// SetBaseURLs overrides base URLs for testing.
func (c *Client) SetBaseURLs(xueqiuBase, stockBase string) {
	c.cfg.XueqiuBase = xueqiuBase
	c.cfg.StockBase = stockBase
}

// SetHasSession marks the client as already having a session (for testing).
func (c *Client) SetHasSession(v bool) {
	c.mu.Lock()
	c.hasSession = v
	c.mu.Unlock()
}

// pace waits until the minimum interval since the last request has elapsed.
func (c *Client) pace() {
	if c.cfg.Rate > 0 {
		if elapsed := time.Since(c.last); elapsed < c.cfg.Rate {
			time.Sleep(c.cfg.Rate - elapsed)
		}
	}
	c.last = time.Now()
}

// bootstrap fetches the Xueqiu session cookie if not already obtained.
// It visits /hq which triggers the server to set the required cookies.
func (c *Client) bootstrap(ctx context.Context) error {
	c.mu.Lock()
	if c.hasSession {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if _, err := c.rawGet(ctx, c.cfg.XueqiuBase+"/hq"); err != nil {
		return fmt.Errorf("xueqiu session bootstrap: %w", err)
	}

	c.mu.Lock()
	c.hasSession = true
	c.mu.Unlock()
	return nil
}

// --- API types ---

// Post is one discussion post from the Xueqiu public timeline.
type Post struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author"`
	ReplyCount  int    `json:"reply_count"`
	LikeCount   int    `json:"like_count"`
	Category    int    `json:"category"`
	URL         string `json:"url"`
}

// Quote is a real-time stock quote from Xueqiu.
type Quote struct {
	Symbol   string  `json:"symbol"`
	Name     string  `json:"name"`
	Current  float64 `json:"current"`
	Percent  float64 `json:"percent"`
	Chg      float64 `json:"chg"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Volume   int64   `json:"volume"`
	Turnover float64 `json:"turnover"`
}

// timelineResponse is the outer envelope from /v4/statuses/public_timeline_by_category.json.
type timelineResponse struct {
	List      []timelineItem `json:"list"`
	NextMaxID int64          `json:"next_max_id"`
	NextID    int64          `json:"next_id"`
}

// timelineItem is one item in the timeline; its Data field is a JSON string.
type timelineItem struct {
	ID       int64  `json:"id"`
	Category int    `json:"category"`
	Data     string `json:"data"`
}

// postData is the inner JSON decoded from timelineItem.Data.
type postData struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ReplyCount  int    `json:"reply_count"`
	LikeCount   int    `json:"like_count"`
	User        struct {
		ScreenName string `json:"screen_name"`
	} `json:"user"`
	Target string `json:"target"`
}

// quoteResponse is the envelope from /query/v1/suggest_stock.json.
type quoteResponse struct {
	Code    int         `json:"code"`
	Data    []quoteData `json:"data"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
}

// quoteData is one stock result from the suggest endpoint.
type quoteData struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Current float64 `json:"current"`
	Percent float64 `json:"percent"`
	Chg     float64 `json:"chg"`
	Open    float64 `json:"open"`
	High    float64 `json:"high"`
	Low     float64 `json:"low"`
	Volume  int64   `json:"volume"`
	Amount  float64 `json:"amount"`
}

// --- API methods ---

// HotPosts fetches trending posts from the Xueqiu public timeline.
func (c *Client) HotPosts(ctx context.Context, limit int) ([]Post, error) {
	if err := c.bootstrap(ctx); err != nil {
		return nil, err
	}

	count := limit
	if count < 1 || count > 20 {
		count = 20
	}
	u := fmt.Sprintf("%s/v4/statuses/public_timeline_by_category.json?since_id=-1&max_id=-1&count=%d&category=-1",
		c.cfg.XueqiuBase, count)

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp timelineResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse timeline: %w", err)
	}

	var posts []Post
	for _, item := range resp.List {
		var pd postData
		if err := json.Unmarshal([]byte(item.Data), &pd); err != nil {
			continue // skip malformed items
		}
		postURL := ""
		if pd.Target != "" {
			postURL = c.cfg.XueqiuBase + pd.Target
		} else {
			postURL = fmt.Sprintf("%s/%d", c.cfg.XueqiuBase, pd.ID)
		}
		posts = append(posts, Post{
			ID:          pd.ID,
			Title:       pd.Title,
			Description: pd.Description,
			Author:      pd.User.ScreenName,
			ReplyCount:  pd.ReplyCount,
			LikeCount:   pd.LikeCount,
			Category:    item.Category,
			URL:         postURL,
		})
	}

	if limit > 0 && limit < len(posts) {
		posts = posts[:limit]
	}
	return posts, nil
}

// StockQuote fetches a real-time quote for the given symbol.
// It uses the suggest_stock endpoint which is accessible with the session cookie.
func (c *Client) StockQuote(ctx context.Context, symbol string) (*Quote, error) {
	if err := c.bootstrap(ctx); err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/query/v1/suggest_stock.json?q=%s&size=1",
		c.cfg.XueqiuBase, url.QueryEscape(symbol))

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp quoteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse quote: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, symbol)
	}

	d := resp.Data[0]
	return &Quote{
		Symbol:   d.Code,
		Name:     d.Name,
		Current:  d.Current,
		Percent:  d.Percent,
		Chg:      d.Chg,
		Open:     d.Open,
		High:     d.High,
		Low:      d.Low,
		Volume:   d.Volume,
		Turnover: d.Amount,
	}, nil
}

// --- internal HTTP helpers ---

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * 500 * time.Millisecond
			if wait > 5*time.Second {
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		b, retry, err := c.doGet(ctx, rawURL)
		if err == nil {
			return b, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

// rawGet is like get but without retry (used for bootstrap).
func (c *Client) rawGet(ctx context.Context, rawURL string) ([]byte, error) {
	b, _, err := c.doGet(ctx, rawURL)
	return b, err
}

func (c *Client) doGet(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.mu.Lock()
	c.pace()
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json, text/html, */*")
	req.Header.Set("Referer", "https://xueqiu.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}
