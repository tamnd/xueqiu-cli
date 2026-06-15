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
  xueqiu hot               show top hot stocks
  xueqiu quote SH600519    quote for Kweichow Moutai
  xueqiu quote AAPL        quote for Apple Inc.`,
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
