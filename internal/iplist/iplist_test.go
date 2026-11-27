package iplist

import (
	"net"
	"testing"
)

func TestIsAutoBanned(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	l := &List{
		autoExact: map[string]bool{"203.0.113.5": true},
		autoCIDR:  []*net.IPNet{cidr},
	}
	if !l.IsAutoBanned("203.0.113.5") {
		t.Fatal("exact auto-ban not matched")
	}
	if !l.IsAutoBanned("192.0.2.99") {
		t.Fatal("cidr auto-ban not matched")
	}
	if l.IsAutoBanned("198.51.100.1") {
		t.Fatal("unexpected auto-ban match")
	}
	if l.IsAutoBanned("not-an-ip") {
		t.Fatal("invalid IP must not match")
	}
}
