package netguard

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

var blocked = []*net.IPNet{
	mustCIDR("0.0.0.0/8"),
	mustCIDR("10.0.0.0/8"),
	mustCIDR("100.64.0.0/10"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("169.254.0.0/16"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.0.0.0/24"),
	mustCIDR("192.0.2.0/24"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("198.18.0.0/15"),
	mustCIDR("198.51.100.0/24"),
	mustCIDR("203.0.113.0/24"),
	mustCIDR("224.0.0.0/4"),
	mustCIDR("240.0.0.0/4"),
	mustCIDR("::1/128"),
	mustCIDR("fc00::/7"),
	mustCIDR("fe80::/10"),
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// IsPrivate reports whether an IP is loopback, private, link-local or another
// range that should not be reachable through a user-controlled upstream.
func IsPrivate(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, n := range blocked {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// hostAllowed checks exact host, wildcard domain (*.example.com) and IP/CIDR
// allowlist entries.
func hostAllowed(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(a), "."))
		if a == "" {
			continue
		}
		if a == host {
			return true
		}
		if strings.HasPrefix(a, "*.") && strings.HasSuffix(host, a[1:]) {
			return true
		}
		if ip != nil {
			if cidr := strings.Trim(a, "[]"); strings.Contains(cidr, "/") {
				if _, n, err := net.ParseCIDR(cidr); err == nil && n.Contains(ip) {
					return true
				}
			} else if aip := net.ParseIP(cidr); aip != nil && aip.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// ValidateURL checks scheme and host of an upstream URL. allowPrivate or an
// allowlist match skip the private-IP test (the returned bool then reports a
// private host that was explicitly allowed). Hostnames are resolved at this
// point so a private A/AAAA record is rejected.
func ValidateURL(raw string, allowPrivate bool, allowed []string) (bool, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("invalid upstream URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false, fmt.Errorf("upstream scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return false, fmt.Errorf("upstream host is required")
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if allowPrivate || hostAllowed(host, allowed) {
			return IsPrivate(ip), nil
		}
		if IsPrivate(ip) {
			return false, fmt.Errorf("upstream host %q resolves to a private or loopback address", host)
		}
		return false, nil
	}
	if allowPrivate || hostAllowed(host, allowed) {
		return false, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, fmt.Errorf("cannot resolve upstream host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return false, fmt.Errorf("upstream host %q has no addresses", host)
	}
	for _, ip := range ips {
		if IsPrivate(ip) {
			return false, fmt.Errorf("upstream host %q resolves to a private or loopback address", host)
		}
	}
	return false, nil
}

// CheckLiteral blocks literal private IPs and "localhost" in the proxy path
// without a DNS lookup. Hostnames were already resolved by ValidateURL when
// the site/location/template was saved; only IP literals are re-checked here.
func CheckLiteral(raw string, allowPrivate bool, allowed []string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid upstream URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("upstream scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("upstream host is required")
	}
	if allowPrivate || hostAllowed(host, allowed) {
		return nil
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("upstream host is not allowed")
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && IsPrivate(ip) {
		return fmt.Errorf("upstream host is not allowed")
	}
	return nil
}
