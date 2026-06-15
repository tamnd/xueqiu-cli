// Package xueqiu is the library behind the xueqiu command line:
// the HTTP client, request shaping, and the typed data models for xueqiu.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, primes the session cookie jar on each call, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package xueqiu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// DefaultUserAgent identifies the client to xueqiu. A real, honest
// User-Agent is both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Host is the site this client talks to.
const Host = "xueqiu.com"

// baseURL is the root used to prime the cookie session.
const baseURL = "https://xueqiu.com"

// stockBase is the root for stock API endpoints.
const stockBase = "https://stock.xueqiu.com"

// Client talks to xueqiu over HTTP using a cookie jar for session management.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	BaseURL   string
	StockBase string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last    time.Time
	primed  bool
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 200ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second, Jar: jar},
		UserAgent: DefaultUserAgent,
		BaseURL:   baseURL,
		StockBase: stockBase,
		Rate:      200 * time.Millisecond,
		Retries:   5,
	}
}

// prime fetches the Xueqiu homepage to set session cookies in the jar.
// It is called once per Client before any API request.
func (c *Client) prime(ctx context.Context) error {
	if c.primed {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	c.primed = true
	return nil
}

// --- wire types ---

type hotResp struct {
	Data struct {
		Data struct {
			Items []struct {
				Code    string  `json:"code"`
				Name    string  `json:"name"`
				Percent float64 `json:"percent"`
				Current float64 `json:"current"`
			} `json:"items"`
		} `json:"data"`
	} `json:"data"`
}

type quoteResp struct {
	Data struct {
		Data struct {
			Quote struct {
				Symbol        string  `json:"symbol"`
				Name          string  `json:"name"`
				Current       float64 `json:"current"`
				Percent       float64 `json:"percent"`
				Chg           float64 `json:"chg"`
				Open          float64 `json:"open"`
				High          float64 `json:"high"`
				Low           float64 `json:"low"`
				Volume        int64   `json:"volume"`
				Amount        float64 `json:"amount"`
				MarketCapital float64 `json:"market_capital"`
				Currency      string  `json:"currency"`
			} `json:"quote"`
		} `json:"data"`
	} `json:"data"`
}

// --- domain types ---

// HotStock is a stock that appears on Xueqiu's hot stocks list.
type HotStock struct {
	Code    string  `json:"code"    kit:"id" table:"symbol"`
	Name    string  `json:"name"             table:"name"`
	Current float64 `json:"current"          table:"price"`
	Percent float64 `json:"percent"          table:"change%"`
}

// Quote is a full stock quote for a single symbol.
type Quote struct {
	Symbol        string  `json:"symbol"         kit:"id" table:"symbol"`
	Name          string  `json:"name"                    table:"name"`
	Current       float64 `json:"current"                 table:"price"`
	Percent       float64 `json:"percent"                 table:"change%"`
	Chg           float64 `json:"chg"                     table:"change"`
	Open          float64 `json:"open"                    table:"open"`
	High          float64 `json:"high"                    table:"high"`
	Low           float64 `json:"low"                     table:"low"`
	Volume        int64   `json:"volume"                  table:"volume"`
	MarketCapital float64 `json:"market_capital"          table:"market_cap"`
	Currency      string  `json:"currency"                table:"currency"`
}

// --- API methods ---

// HotStocks returns the top N hot stocks from Xueqiu community.
func (c *Client) HotStocks(ctx context.Context, size int) ([]*HotStock, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if size <= 0 {
		size = 20
	}
	url := fmt.Sprintf("%s/v5/stock/hot_stock/list.json?size=%d&_type=10&type=10", c.StockBase, size)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp hotResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode hot stocks: %w", err)
	}
	items := resp.Data.Data.Items
	out := make([]*HotStock, 0, len(items))
	for _, it := range items {
		out = append(out, &HotStock{
			Code:    it.Code,
			Name:    it.Name,
			Current: it.Current,
			Percent: it.Percent,
		})
	}
	return out, nil
}

// GetQuote returns a full stock quote for the given symbol (e.g. "SH600519", "AAPL").
func (c *Client) GetQuote(ctx context.Context, symbol string) (*Quote, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	url := fmt.Sprintf("%s/v5/stock/quote.json?symbol=%s&extend=detail", c.StockBase, symbol)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp quoteResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode quote: %w", err)
	}
	q := resp.Data.Data.Quote
	return &Quote{
		Symbol:        q.Symbol,
		Name:          q.Name,
		Current:       q.Current,
		Percent:       q.Percent,
		Chg:           q.Chg,
		Open:          q.Open,
		High:          q.High,
		Low:           q.Low,
		Volume:        q.Volume,
		MarketCapital: q.MarketCapital,
		Currency:      q.Currency,
	}, nil
}

// get fetches url and returns the response body with pacing and retries.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", c.BaseURL)

	resp, err := c.HTTP.Do(req)
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
