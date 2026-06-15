package xueqiu

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in xueqiu_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "xueqiu" {
		t.Errorf("Scheme = %q, want xueqiu", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "xueqiu" {
		t.Errorf("Identity.Binary = %q, want xueqiu", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"SH600519", "quote", "SH600519"},
		{"aapl", "quote", "AAPL"},
		{"sz000858", "quote", "SZ000858"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("quote", "SH600519")
	want := "https://xueqiu.com/S/SH600519"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}

// TestHostWiring mounts the driver in a kit Host (the runtime ant drives) and
// checks the round trip: a record mints to its URI, its body is readable, and a
// bare id resolves back to the same URI.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	q := &Quote{Symbol: "SH600519", Name: "贵州茅台", Current: 1800.00, Currency: "CNY"}
	u, err := h.Mint(q)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "xueqiu://quote/SH600519"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("xueqiu", "AAPL")
	if err != nil || got.String() != "xueqiu://quote/AAPL" {
		t.Errorf("ResolveOn = (%q, %v), want xueqiu://quote/AAPL", got.String(), err)
	}
}
