// Package xueqiu exposes the Xueqiu kit Domain.
package xueqiu

import (
	"context"
	"errors"
	"fmt"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Xueqiu driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, hosts, and identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "xueqiu",
		Hosts:  []string{Host, "www.xueqiu.com", "stock.xueqiu.com"},
		Identity: kit.Identity{
			Binary: "xue",
			Short:  "Browse Xueqiu stock discussions and quotes",
			Long: `xue turns xueqiu.com into a fast, scriptable command line.

Read trending investment discussions and real-time stock quotes from the public
Xueqiu APIs. A session cookie is obtained automatically on first use, no account
or API key required.

Quick start:
  xue hot                   top 10 trending posts
  xue hot -n 20             top 20 posts
  xue quote SH000001        Shanghai Composite Index
  xue quote AAPL            Apple Inc
  xue quote HK00700         Tencent Holdings
  xue hot -o jsonl          posts as newline-delimited JSON`,
			Site: Host,
			Repo: "https://github.com/tamnd/xueqiu-cli",
		},
	}
}

// Register installs the client factory and the xueqiu operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "hot",
		Group:   "posts",
		Summary: "List trending posts and discussions on Xueqiu",
	}, listHot)

	kit.Handle(app, kit.OpMeta{
		Name:     "quote",
		Group:    "stocks",
		Single:   true,
		Summary:  "Fetch a real-time stock quote",
		Args:     []kit.Arg{{Name: "symbol", Help: "stock symbol (e.g. SH000001, AAPL, HK00700)"}},
	}, getQuote)
}

// newClient builds a Client from the resolved kit Config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.cfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.cfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.cfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.cfg.Timeout = cfg.Timeout
		c.http.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type hotInput struct {
	Limit  int     `kit:"flag,inherit" help:"max posts to show" default:"10"`
	Client *Client `kit:"inject"`
}

type quoteInput struct {
	Symbol string  `kit:"arg" help:"stock symbol (e.g. SH000001, AAPL)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listHot(ctx context.Context, in hotInput, emit func(Post) error) error {
	posts, err := in.Client.HotPosts(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range posts {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func getQuote(ctx context.Context, in quoteInput, emit func(*Quote) error) error {
	if in.Symbol == "" {
		return errs.Usage("xue quote: symbol is required")
	}
	q, err := in.Client.StockQuote(ctx, in.Symbol)
	if err != nil {
		return mapErr(err)
	}
	return emit(q)
}

// --- Resolver ---

// Classify turns any input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("xueqiu: empty input")
	}
	return "stock", input, nil
}

// Locate returns the canonical URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "stock":
		return fmt.Sprintf("https://xueqiu.com/S/%s", id), nil
	default:
		return "", errs.Usage("xueqiu has no resource type %q", uriType)
	}
}

// mapErr converts library errors to kit error types.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	return err
}
