package xueqiu

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes xueqiu as a kit Domain so the binary and a multi-domain
// host (ant) share one source of truth. The init below registers it; the host
// then dereferences xueqiu:// URIs by routing to the operations installed here.
func init() { kit.Register(Domain{}) }

// Domain is the xueqiu driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "xueqiu",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "xueqiu",
			Short:  "A command line for Xueqiu stock community.",
			Long: `A command line for Xueqiu stock community.

xueqiu reads public Xueqiu data over plain HTTPS, shapes it into clean records,
and prints output that pipes into the rest of your tools. No API key required.

Examples:
  xueqiu hot                    show top hot stocks
  xueqiu quote SH600519         quote for Kweichow Moutai
  xueqiu kline SH600519         100-day OHLCV history
  xueqiu stocks --market CN     list A-share stocks
  xueqiu screener --pe-max 20   screen by PE ratio
  xueqiu news SH600519          stock news feed
  xueqiu comments SH600519      community posts about a stock`,
			Site: Host,
			Repo: "https://github.com/tamnd/xueqiu-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// hot: list the top hot stocks from the Xueqiu community.
	kit.Handle(app, kit.OpMeta{
		Name:    "hot",
		Group:   "read",
		List:    true,
		Summary: "List hot stocks on Xueqiu",
	}, listHot)

	// quote: fetch a full stock quote for one symbol.
	kit.Handle(app, kit.OpMeta{
		Name:     "quote",
		Group:    "read",
		Single:   true,
		Summary:  "Fetch a stock quote by symbol",
		URIType:  "quote",
		Resolver: true,
		Args:     []kit.Arg{{Name: "symbol", Help: "stock symbol, e.g. SH600519 or AAPL"}},
	}, getQuote)

	// kline: OHLCV price history for a symbol.
	kit.Handle(app, kit.OpMeta{
		Name:    "kline",
		Group:   "read",
		List:    true,
		Summary: "Fetch OHLCV price history for a stock",
		Args:    []kit.Arg{{Name: "symbol", Help: "stock symbol, e.g. SH600519 or AAPL"}},
	}, listKline)

	// stocks: paginated stock list for a market.
	kit.Handle(app, kit.OpMeta{
		Name:    "stocks",
		Group:   "read",
		List:    true,
		Summary: "List stocks for a market (CN, US, HK)",
	}, listStocks)

	// screener: filter stocks by fundamental criteria.
	kit.Handle(app, kit.OpMeta{
		Name:    "screener",
		Group:   "read",
		List:    true,
		Summary: "Screen stocks by market and PE ratio",
	}, listScreener)

	// news: stock-specific news and post timeline.
	kit.Handle(app, kit.OpMeta{
		Name:    "news",
		Group:   "read",
		List:    true,
		Summary: "Fetch the news/post timeline for a stock",
		Args:    []kit.Arg{{Name: "symbol", Help: "stock symbol, e.g. SH600519"}},
	}, listNews)

	// comments: community posts mentioning a stock.
	kit.Handle(app, kit.OpMeta{
		Name:    "comments",
		Group:   "read",
		List:    true,
		Summary: "Fetch community posts mentioning a stock",
		Args:    []kit.Arg{{Name: "symbol", Help: "stock symbol, e.g. SH600519"}},
	}, listComments)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type hotIn struct {
	Limit  int     `kit:"flag,inherit" help:"max results (default 20)"`
	Client *Client `kit:"inject"`
}

type quoteIn struct {
	Symbol string  `kit:"arg" help:"stock symbol, e.g. SH600519 or AAPL"`
	Client *Client `kit:"inject"`
}

type klineIn struct {
	Symbol string  `kit:"arg"          help:"stock symbol, e.g. SH600519 or AAPL"`
	Period string  `kit:"flag"         help:"candle period: day, week, month (default day)"`
	Count  int     `kit:"flag"         help:"number of candles, negative = most recent (default -100)"`
	Client *Client `kit:"inject"`
}

type stocksIn struct {
	Market string  `kit:"flag"         help:"market: CN, US, HK (default CN)"`
	Page   int     `kit:"flag"         help:"page number (default 1)"`
	Client *Client `kit:"inject"`
}

type screenerIn struct {
	Market string  `kit:"flag"         help:"market: CN, US, HK (default CN)"`
	PeMin  float64 `kit:"flag"         help:"minimum PE ratio (TTM)"`
	PeMax  float64 `kit:"flag"         help:"maximum PE ratio (TTM)"`
	Page   int     `kit:"flag"         help:"page number (default 1)"`
	Client *Client `kit:"inject"`
}

type newsIn struct {
	Symbol string  `kit:"arg"          help:"stock symbol, e.g. SH600519"`
	Count  int     `kit:"flag,inherit" help:"max results (default 15)"`
	Client *Client `kit:"inject"`
}

type commentsIn struct {
	Symbol string  `kit:"arg"          help:"stock symbol, e.g. SH600519"`
	Count  int     `kit:"flag,inherit" help:"max results per page (default 15)"`
	Page   int     `kit:"flag"         help:"page number (default 1)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listHot(ctx context.Context, in hotIn, emit func(*HotStock) error) error {
	items, err := in.Client.HotStocks(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}

func getQuote(ctx context.Context, in quoteIn, emit func(*Quote) error) error {
	q, err := in.Client.GetQuote(ctx, strings.ToUpper(in.Symbol))
	if err != nil {
		return mapErr(err)
	}
	return emit(q)
}

func listKline(ctx context.Context, in klineIn, emit func(*Candle) error) error {
	candles, err := in.Client.Kline(ctx, strings.ToUpper(in.Symbol), in.Period, in.Count)
	if err != nil {
		return mapErr(err)
	}
	for _, c := range candles {
		if err := emit(c); err != nil {
			return err
		}
	}
	return nil
}

func listStocks(ctx context.Context, in stocksIn, emit func(*Stock) error) error {
	stocks, err := in.Client.Stocks(ctx, strings.ToUpper(in.Market), in.Page)
	if err != nil {
		return mapErr(err)
	}
	for _, s := range stocks {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func listScreener(ctx context.Context, in screenerIn, emit func(*Stock) error) error {
	stocks, err := in.Client.Screener(ctx, strings.ToUpper(in.Market), in.PeMin, in.PeMax, in.Page)
	if err != nil {
		return mapErr(err)
	}
	for _, s := range stocks {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func listNews(ctx context.Context, in newsIn, emit func(*NewsItem) error) error {
	items, err := in.Client.News(ctx, strings.ToUpper(in.Symbol), in.Count)
	if err != nil {
		return mapErr(err)
	}
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}

func listComments(ctx context.Context, in commentsIn, emit func(*Comment) error) error {
	items, err := in.Client.Comments(ctx, strings.ToUpper(in.Symbol), in.Count, in.Page)
	if err != nil {
		return mapErr(err)
	}
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: pure string functions, network-free ---

// Classify turns a symbol into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	s := strings.TrimSpace(strings.ToUpper(input))
	if s == "" {
		return "", "", errs.Usage("unrecognized xueqiu reference: %q", input)
	}
	return "quote", s, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "quote":
		return "https://xueqiu.com/S/" + id, nil
	default:
		return "", errs.Usage("xueqiu has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	return err
}
