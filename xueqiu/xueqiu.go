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

type klineResp struct {
	Data struct {
		Symbol string   `json:"symbol"`
		Column []string `json:"column"`
		Item   [][]any  `json:"item"`
	} `json:"data"`
}

type stocksResp struct {
	Data struct {
		List []struct {
			Symbol  string  `json:"symbol"`
			Name    string  `json:"name"`
			Current float64 `json:"current"`
			Percent float64 `json:"percent"`
			Volume  int64   `json:"volume"`
			Amount  float64 `json:"amount"`
		} `json:"list"`
	} `json:"data"`
}

type screenerResp struct {
	Data struct {
		List []struct {
			Symbol  string  `json:"symbol"`
			Name    string  `json:"name"`
			Current float64 `json:"current"`
			Percent float64 `json:"percent"`
			Volume  int64   `json:"volume"`
			Amount  float64 `json:"amount"`
			PeTTM   float64 `json:"pe_ttm"`
		} `json:"list"`
	} `json:"data"`
}

type newsResp struct {
	Statuses []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		CreatedAt   int64  `json:"created_at"`
		ReplyCount  int    `json:"reply_count"`
		FavCount    int    `json:"fav_count"`
	} `json:"statuses"`
}

type commentsResp struct {
	Statuses []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		CreatedAt   int64  `json:"created_at"`
		ReplyCount  int    `json:"reply_count"`
		FavCount    int    `json:"fav_count"`
		User        struct {
			ID             int64  `json:"id"`
			ScreenName     string `json:"screen_name"`
			FollowersCount int    `json:"followers_count"`
		} `json:"user"`
	} `json:"statuses"`
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

// Candle is one OHLCV bar from the kline endpoint.
type Candle struct {
	Symbol    string  `json:"symbol"    kit:"id" table:"symbol"`
	Timestamp int64   `json:"timestamp"          table:"timestamp"`
	Open      float64 `json:"open"               table:"open"`
	High      float64 `json:"high"               table:"high"`
	Low       float64 `json:"low"                table:"low"`
	Close     float64 `json:"close"              table:"close"`
	Volume    int64   `json:"volume"             table:"volume"`
	Amount    float64 `json:"amount"             table:"amount"`
}

// Stock is one entry from the stock list or screener.
type Stock struct {
	Symbol  string  `json:"symbol"  kit:"id" table:"symbol"`
	Name    string  `json:"name"             table:"name"`
	Current float64 `json:"current"          table:"price"`
	Percent float64 `json:"percent"          table:"change%"`
	Volume  int64   `json:"volume"           table:"volume"`
	Amount  float64 `json:"amount"           table:"amount"`
	PeTTM   float64 `json:"pe_ttm"           table:"pe_ttm"`
}

// NewsItem is a post or news article from the stock timeline.
type NewsItem struct {
	ID          int64  `json:"id"          kit:"id" table:"id"`
	Title       string `json:"title"                table:"title"`
	Description string `json:"description"          table:"description"`
	CreatedAt   int64  `json:"created_at"           table:"created_at"`
	ReplyCount  int    `json:"reply_count"          table:"replies"`
	FavCount    int    `json:"fav_count"            table:"favs"`
}

// Comment is a community post about a stock from the search/query endpoint.
type Comment struct {
	ID             int64  `json:"id"              kit:"id" table:"id"`
	Title          string `json:"title"                    table:"title"`
	Description    string `json:"description"              table:"description"`
	CreatedAt      int64  `json:"created_at"               table:"created_at"`
	ReplyCount     int    `json:"reply_count"              table:"replies"`
	FavCount       int    `json:"fav_count"                table:"favs"`
	UserID         int64  `json:"user_id"                  table:"user_id"`
	UserName       string `json:"user_name"                table:"user"`
	FollowersCount int    `json:"followers_count"          table:"followers"`
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

// Kline returns OHLCV candles for a symbol. count is negative (e.g. -100) to
// retrieve the most recent N bars. period is "day", "week", or "month".
func (c *Client) Kline(ctx context.Context, symbol, period string, count int) ([]*Candle, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if period == "" {
		period = "day"
	}
	if count == 0 {
		count = -100
	}
	url := fmt.Sprintf("%s/v5/stock/chart/kline.json?symbol=%s&begin=%d&period=%s&type=before_adj&count=%d",
		c.StockBase, symbol, nowMS(), period, count)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp klineResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode kline: %w", err)
	}
	// column order: timestamp,volume,open,high,low,close,chg,percent,turnoverrate,amount,...
	colIdx := make(map[string]int, len(resp.Data.Column))
	for i, col := range resp.Data.Column {
		colIdx[col] = i
	}
	out := make([]*Candle, 0, len(resp.Data.Item))
	for _, row := range resp.Data.Item {
		c := &Candle{Symbol: resp.Data.Symbol}
		if i, ok := colIdx["timestamp"]; ok && i < len(row) {
			c.Timestamp = int64Val(row[i])
		}
		if i, ok := colIdx["open"]; ok && i < len(row) {
			c.Open = floatVal(row[i])
		}
		if i, ok := colIdx["high"]; ok && i < len(row) {
			c.High = floatVal(row[i])
		}
		if i, ok := colIdx["low"]; ok && i < len(row) {
			c.Low = floatVal(row[i])
		}
		if i, ok := colIdx["close"]; ok && i < len(row) {
			c.Close = floatVal(row[i])
		}
		if i, ok := colIdx["volume"]; ok && i < len(row) {
			c.Volume = int64Val(row[i])
		}
		if i, ok := colIdx["amount"]; ok && i < len(row) {
			c.Amount = floatVal(row[i])
		}
		out = append(out, c)
	}
	return out, nil
}

// Stocks returns a paginated list of stocks for a market.
// market is "CN", "US", or "HK".
func (c *Client) Stocks(ctx context.Context, market string, page int) ([]*Stock, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if market == "" {
		market = "CN"
	}
	if page <= 0 {
		page = 1
	}
	url := fmt.Sprintf("%s/v5/stock/screener/quote/list.json?market=%s&order=desc&order_by=amount&page=%d&size=90&type=11",
		c.StockBase, market, page)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp stocksResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode stocks: %w", err)
	}
	out := make([]*Stock, 0, len(resp.Data.List))
	for _, it := range resp.Data.List {
		out = append(out, &Stock{
			Symbol:  it.Symbol,
			Name:    it.Name,
			Current: it.Current,
			Percent: it.Percent,
			Volume:  it.Volume,
			Amount:  it.Amount,
		})
	}
	return out, nil
}

// Screener returns stocks matching optional PE ratio filters.
// market is "CN", "US", or "HK"; peMin/peMax of 0 means no filter.
func (c *Client) Screener(ctx context.Context, market string, peMin, peMax float64, page int) ([]*Stock, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if market == "" {
		market = "CN"
	}
	if page <= 0 {
		page = 1
	}
	pe := "ALL"
	if peMin > 0 || peMax > 0 {
		if peMax > 0 {
			pe = fmt.Sprintf("%g_%g", peMin, peMax)
		} else {
			pe = fmt.Sprintf("%g_", peMin)
		}
	}
	url := fmt.Sprintf("%s/v5/stock/screener/quote/list.json?market=%s&order=desc&order_by=percent&page=%d&size=20&type=11&pe_ttm=%s",
		c.StockBase, market, page, pe)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp screenerResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode screener: %w", err)
	}
	out := make([]*Stock, 0, len(resp.Data.List))
	for _, it := range resp.Data.List {
		out = append(out, &Stock{
			Symbol:  it.Symbol,
			Name:    it.Name,
			Current: it.Current,
			Percent: it.Percent,
			Volume:  it.Volume,
			Amount:  it.Amount,
			PeTTM:   it.PeTTM,
		})
	}
	return out, nil
}

// News returns the news/post timeline for a stock symbol.
func (c *Client) News(ctx context.Context, symbol string, count int) ([]*NewsItem, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if count <= 0 {
		count = 15
	}
	url := fmt.Sprintf("%s/statuses/stock_timeline.json?symbol_id=%s&count=%d&source=timeline",
		c.BaseURL, symbol, count)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp newsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode news: %w", err)
	}
	out := make([]*NewsItem, 0, len(resp.Statuses))
	for _, s := range resp.Statuses {
		out = append(out, &NewsItem{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			CreatedAt:   s.CreatedAt,
			ReplyCount:  s.ReplyCount,
			FavCount:    s.FavCount,
		})
	}
	return out, nil
}

// Comments returns community posts mentioning a stock symbol via the search endpoint.
func (c *Client) Comments(ctx context.Context, symbol string, count, page int) ([]*Comment, error) {
	if err := c.prime(ctx); err != nil {
		return nil, fmt.Errorf("prime: %w", err)
	}
	if count <= 0 {
		count = 15
	}
	if page <= 0 {
		page = 1
	}
	url := fmt.Sprintf("%s/query.json?q=%%24%s&count=%d&page=%d",
		c.BaseURL, symbol, count, page)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var resp commentsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode comments: %w", err)
	}
	out := make([]*Comment, 0, len(resp.Statuses))
	for _, s := range resp.Statuses {
		out = append(out, &Comment{
			ID:             s.ID,
			Title:          s.Title,
			Description:    s.Description,
			CreatedAt:      s.CreatedAt,
			ReplyCount:     s.ReplyCount,
			FavCount:       s.FavCount,
			UserID:         s.User.ID,
			UserName:       s.User.ScreenName,
			FollowersCount: s.User.FollowersCount,
		})
	}
	return out, nil
}

// nowMS returns the current time in milliseconds since epoch.
func nowMS() int64 {
	return time.Now().UnixMilli()
}

// floatVal converts an any JSON number (float64 from json.Unmarshal) to float64.
func floatVal(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

// int64Val converts an any JSON number to int64.
func int64Val(v any) int64 {
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	return 0
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
