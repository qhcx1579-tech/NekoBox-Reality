package libcore

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseExitIP(t *testing.T) {
	for body, expected := range map[string]string{
		`{"ip":"1.1.1.1","countryCode":"AU","asn":13335,"city":"ignored"}`: "1.1.1.1\nAU",
		`{"ip":"2606:4700:4700::1111","countryCode":"US"}`:                 "2606:4700:4700::1111\nUS",
	} {
		if result, err := parseExitIP(body); err != nil || result != expected {
			t.Fatal("response must contain only validated IP and country")
		}
	}
	for _, body := range []string{
		`<html>error</html>`, `{"ip":"127.0.0.1","countryCode":"US"}`,
		`{"ip":"10.0.0.1","countryCode":"US"}`, `{"ip":"example.com","countryCode":"US"}`,
		`{"ip":"1.1.1.1","countryCode":"bad"}`, `{"ip":"::1","countryCode":"US"}`,
		`{"ip":"fc00::1","countryCode":"US"}`, `{"ip":"::ffff:127.0.0.1","countryCode":"US"}`,
		`{"ip":"1.1.1.1"}`, `{"ip":"1.1.1.1","countryCode":123}`, `null`, `[]`,
	} {
		if _, err := parseExitIP(body); err == nil {
			t.Fatal("accepted invalid response")
		}
	}
}

type localExitIPTransport struct {
	base   http.RoundTripper
	server *url.URL
	t      *testing.T
	calls  int
}

// Redirect only the fixed API request into a real local TLS server for tests.
func (r *localExitIPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	if req.URL.String() != exitIPURL || req.Method != http.MethodGet || req.Header.Get("Accept") != "application/json" {
		r.t.Fatal("unexpected API request")
	}
	copy := req.Clone(req.Context())
	copy.URL.Scheme, copy.URL.Host = r.server.Scheme, r.server.Host
	return r.base.RoundTrip(copy)
}

func TestFetchExitIP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		ok     bool
	}{
		{"success", 200, `{"ip":"1.1.1.1","countryCode":"AU","fraudScore":99}`, true},
		{"rate_limited", 429, "private server message", false},
		{"malformed", 200, "<html>unavailable</html>", false},
		{"oversized", 200, strings.Repeat("x", 4097), false},
		{"redirect", 302, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/v1/info" {
					t.Error("wrong API path")
				}
				w.Header().Set("Location", "http://127.0.0.1/unwanted")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			localURL, _ := url.Parse(server.URL)
			client := server.Client()
			transport := &localExitIPTransport{base: client.Transport, server: localURL, t: t}
			client.Transport = transport
			result, err := fetchExitIP(client)
			if tc.ok {
				if err != nil || result != "1.1.1.1\nAU" {
					t.Fatal("valid API result failed")
				}
			} else if err == nil || result != "" || strings.Contains(err.Error(), tc.body) && tc.body != "" {
				t.Fatal("failure must return a fixed error, without a remote body")
			}
			if transport.calls != 1 {
				t.Fatal("unexpected retry or redirect")
			}
		})
	}
}
