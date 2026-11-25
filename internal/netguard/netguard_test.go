package netguard

import "testing"

func TestCheckLiteralBlocksPrivate(t *testing.T) {
	if err := CheckLiteral("http://169.254.169.254/latest/meta-data/", false, nil); err == nil {
		t.Error("metadata IP should be blocked")
	}
	if err := CheckLiteral("http://127.0.0.1:8080/", false, nil); err == nil {
		t.Error("loopback should be blocked")
	}
	if err := CheckLiteral("http://localhost/", false, nil); err == nil {
		t.Error("localhost should be blocked")
	}
	if err := CheckLiteral("http://10.0.0.5/", false, nil); err == nil {
		t.Error("private range should be blocked")
	}
}

func TestCheckLiteralAllowsConfigured(t *testing.T) {
	if err := CheckLiteral("http://10.10.20.188/", false, []string{"10.10.20.0/24"}); err != nil {
		t.Errorf("allowed CIDR should pass: %v", err)
	}
	if err := CheckLiteral("http://192.168.1.5/", true, nil); err != nil {
		t.Errorf("allowPrivate should pass: %v", err)
	}
	if err := CheckLiteral("http://internal.example.com/", false, []string{"*.example.com"}); err != nil {
		t.Errorf("wildcard domain should pass: %v", err)
	}
}

func TestCheckLiteralScheme(t *testing.T) {
	if err := CheckLiteral("file:///etc/passwd", true, nil); err == nil {
		t.Error("file scheme must be rejected")
	}
	if err := CheckLiteral("gopher://1.2.3.4:1111/", true, nil); err == nil {
		t.Error("gopher scheme must be rejected")
	}
}

func TestValidateURLIPs(t *testing.T) {
	if ok, err := ValidateURL("http://127.0.0.1/", true, nil); err != nil || !ok {
		t.Errorf("private-allowed should return ok=true,err=nil, got %v %v", ok, err)
	}
	if _, err := ValidateURL("http://127.0.0.1/", false, nil); err == nil {
		t.Error("loopback must be rejected without allow")
	}
	if _, err := ValidateURL("http://[::1]/", false, nil); err == nil {
		t.Error("IPv6 loopback must be rejected")
	}
}

func TestHostAllowedCIDRAndExact(t *testing.T) {
	if !hostAllowed("10.10.20.5", []string{"10.10.20.0/24"}) {
		t.Error("CIDR match failed")
	}
	if !hostAllowed("sub.Example.COM", []string{"*.example.com"}) {
		t.Error("wildcard case-insensitive match failed")
	}
	if hostAllowed("10.11.0.1", []string{"10.10.20.0/24"}) {
		t.Error("out-of-range should not match")
	}
}

func TestCheckLiveLiterals(t *testing.T) {
	if err := CheckLive("http://169.254.169.254/latest", false, nil); err == nil {
		t.Error("metadata IP must be blocked")
	}
	if err := CheckLive("http://127.0.0.1:8080/", false, nil); err == nil {
		t.Error("loopback must be blocked")
	}
	if err := CheckLive("http://8.8.8.8/", false, nil); err != nil {
		t.Errorf("public literal should pass: %v", err)
	}
	if err := CheckLive("http://10.10.20.188/", false, []string{"10.10.20.0/24"}); err != nil {
		t.Errorf("allowlisted CIDR should pass: %v", err)
	}
	if err := CheckLive("http://localhost/", false, nil); err == nil {
		t.Error("localhost must be blocked")
	}
	if err := CheckLive("http://localhost/", false, []string{"localhost"}); err != nil {
		t.Errorf("allowlisted hostname should pass without DNS: %v", err)
	}
	if err := CheckLive("file:///etc/passwd", true, nil); err == nil {
		t.Error("file scheme must be rejected")
	}
}

func TestValidateURLHostnameAllowlistIsTrusted(t *testing.T) {
	// An allowlisted hostname must report privateAllowed=true so the proxy
	// path trusts it and never re-blocks it after a DNS change.
	ok, err := ValidateURL("http://db.internal.example/", false, []string{"*.internal.example"})
	if err != nil || !ok {
		t.Errorf("expected (true, nil), got (%v, %v)", ok, err)
	}
}
