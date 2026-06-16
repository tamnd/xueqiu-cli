package xueqiu

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and host wiring. The client HTTP behaviour is covered in xueqiu_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "xueqiu" {
		t.Errorf("Scheme = %q, want xueqiu", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "xue" {
		t.Errorf("Identity.Binary = %q, want xue", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	typ, id, err := Domain{}.Classify("SH000001")
	if err != nil || typ != "stock" || id != "SH000001" {
		t.Errorf("Classify = (%q, %q, %v), want (stock, SH000001, nil)", typ, id, err)
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify empty input: want error, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("stock", "SH000001")
	want := "https://xueqiu.com/S/SH000001"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "id")
	if err == nil {
		t.Error("Locate unknown type: want error, got nil")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks registration.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("xueqiu", "AAPL")
	if err != nil || got.String() != "xueqiu://stock/AAPL" {
		t.Errorf("ResolveOn = (%q, %v), want xueqiu://stock/AAPL", got.String(), err)
	}
}
