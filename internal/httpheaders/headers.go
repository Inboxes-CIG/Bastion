package httpheaders

import (
	"fmt"
	"net/http"
	"strings"
)

// blockedHeaders must not be overridden from user input: they are either
// hop-by-hop, control framing, or would conflict with the proxy's own
// compression/transport handling.
var blockedHeaders = map[string]bool{
	"connection":          true,
	"keep-alive":          true,
	"proxy-authenticate":  true,
	"proxy-authorization": true,
	"te":                  true,
	"trailer":             true,
	"transfer-encoding":   true,
	"upgrade":             true,
	"content-length":      true,
	"content-encoding":    true,
	"host":                true,
}

// Sanitize validates and canonicalizes a map of response headers.
func Sanitize(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return in, nil
	}
	out := make(map[string]string, len(in))
	for rawName, rawValue := range in {
		name := http.CanonicalHeaderKey(strings.TrimSpace(rawName))
		value := strings.TrimSpace(rawValue)
		if name == "" {
			continue
		}
		if !validName(name) {
			return nil, fmt.Errorf("invalid header name %q", rawName)
		}
		if blockedHeaders[strings.ToLower(name)] {
			return nil, fmt.Errorf("header %q may not be overridden", name)
		}
		if !validValue(value) {
			return nil, fmt.Errorf("invalid value for header %q", name)
		}
		out[name] = value
	}
	return out, nil
}

// Apply sets the given headers on h, overriding upstream values.
func Apply(h http.Header, custom map[string]string) {
	for k, v := range custom {
		h.Set(k, v)
	}
}

func validName(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		}
		return false
	}
	return true
}

func validValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '\r' || c == '\n' || c == 0x7f {
			return false
		}
		if c < 0x20 && c != '\t' {
			return false
		}
	}
	return true
}
