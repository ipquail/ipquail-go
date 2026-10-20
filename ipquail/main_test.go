package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func trustedRequest(method, target string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set(realIPHeader, "203.0.113.10")
	return r
}

func assertResponse(t *testing.T, w *httptest.ResponseRecorder, status int, body string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d", w.Code, status)
	}
	if got := w.Body.String(); got != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func assertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, key, value string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body[key]; got != value {
		t.Errorf("%s = %q, want %q", key, got, value)
	}
}

func TestNewApplicationUsesSystemResolvers(t *testing.T) {
	app := newApplication()
	if app.lookupAddr == nil {
		t.Error("lookupAddr is nil")
	}
	if app.lookupTXT == nil {
		t.Error("lookupTXT is nil")
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{name: "empty", input: []int{}, want: []int{}},
		{name: "single element", input: []int{1}, want: []int{1}},
		{name: "even length", input: []int{1, 2, 3, 4}, want: []int{4, 3, 2, 1}},
		{name: "odd length", input: []int{1, 2, 3, 4, 5}, want: []int{5, 4, 3, 2, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := append(make([]int, 0, len(tt.input)), tt.input...)
			if got := Reverse(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Reverse(%v) = %v, want %v", tt.input, got, tt.want)
			}
			if !reflect.DeepEqual(tt.input, original) {
				t.Fatalf("Reverse modified input: got %v, want %v", tt.input, original)
			}
		})
	}
}

func TestIPHandlerUsesTrustedRealIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		realIP     string
		wantStatus int
		wantBody   string
	}{
		{name: "IPv4", remoteAddr: "127.0.0.1:12345", realIP: "203.0.113.10", wantStatus: http.StatusOK, wantBody: "203.0.113.10\n"},
		{name: "IPv6", remoteAddr: "[::1]:12345", realIP: "2001:db8::10", wantStatus: http.StatusOK, wantBody: "2001:db8::10\n"},
		{name: "IPv4-mapped address is normalized", remoteAddr: "127.0.0.1:12345", realIP: "::ffff:203.0.113.10", wantStatus: http.StatusOK, wantBody: "203.0.113.10\n"},
		{name: "malformed remote address", remoteAddr: "not-an-address", realIP: "203.0.113.10", wantStatus: http.StatusBadRequest, wantBody: "invalid remote address\n"},
		{name: "missing trusted header", remoteAddr: "127.0.0.1:12345", wantStatus: http.StatusBadRequest, wantBody: "invalid client address\n"},
		{name: "invalid trusted header", remoteAddr: "127.0.0.1:12345", realIP: "invalid", wantStatus: http.StatusBadRequest, wantBody: "invalid client address\n"},
		{name: "multiple addresses rejected", remoteAddr: "127.0.0.1:12345", realIP: "203.0.113.10, 198.51.100.20", wantStatus: http.StatusBadRequest, wantBody: "invalid client address\n"},
		{name: "private-network proxy accepted", remoteAddr: "172.18.0.2:12345", realIP: "203.0.113.10", wantStatus: http.StatusOK, wantBody: "203.0.113.10\n"},
		{name: "non-loopback peer rejected", remoteAddr: "192.0.2.10:12345", realIP: "203.0.113.10", wantStatus: http.StatusForbidden, wantBody: "forbidden\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/ip", nil)
			r.RemoteAddr = tt.remoteAddr
			r.Header.Set(realIPHeader, tt.realIP)
			r.Header.Set("X-Forwarded-For", "attacker-controlled.invalid")
			w := httptest.NewRecorder()
			ipHandler(w, r)
			assertResponse(t, w, tt.wantStatus, tt.wantBody)
			if tt.wantStatus == http.StatusOK && w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Errorf("unexpected Content-Type %q", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestASNQuery(t *testing.T) {
	tests := []struct {
		name string
		ip   netip.Addr
		want string
	}{
		{name: "IPv4 aggregates to 24 bits", ip: netip.MustParseAddr("203.0.113.10"), want: "113.0.203.origin.asn.cymru.com"},
		{name: "global IPv6 aggregates to 64 bits", ip: netip.MustParseAddr("2001:db8:1234::1"), want: "0.0.0.0.4.3.2.1.8.b.d.0.1.0.0.2.origin6.asn.cymru.com"},
		{name: "leading-zero IPv6 remains fixed width", ip: netip.MustParseAddr("::1"), want: "0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.origin6.asn.cymru.com"},
		{name: "invalid address", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := asnQuery(tt.ip); got != tt.want {
				t.Fatalf("asnQuery(%q) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}

func TestHealthHandlerDoesNotRequireClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.RemoteAddr = "192.0.2.10:12345"
	w := httptest.NewRecorder()

	healthHandler(w, r)

	assertResponse(t, w, http.StatusOK, "ok\n")
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
}

func TestPTRHandler(t *testing.T) {
	tests := []struct {
		name       string
		addresses  []string
		lookupErr  error
		reject     bool
		wantStatus int
		wantBody   string
	}{
		{name: "successful lookup", addresses: []string{"host.example."}, wantStatus: http.StatusOK, wantBody: "host.example.\n"},
		{name: "empty lookup", wantStatus: http.StatusOK, wantBody: "none\n"},
		{name: "lookup error", lookupErr: errors.New("DNS failed"), wantStatus: http.StatusOK, wantBody: "none\n"},
		{name: "rejected request", reject: true, wantStatus: http.StatusForbidden, wantBody: "forbidden\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{lookupAddr: func(address string) ([]string, error) {
				if address != "203.0.113.10" {
					t.Errorf("lookup address = %q", address)
				}
				return tt.addresses, tt.lookupErr
			}}
			r := trustedRequest(http.MethodGet, "/ptr")
			if tt.reject {
				r.RemoteAddr = "192.0.2.1:12345"
			}
			w := httptest.NewRecorder()
			app.ptrHandler(w, r)
			assertResponse(t, w, tt.wantStatus, tt.wantBody)
			if !tt.reject && w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Errorf("unexpected Content-Type %q", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestASNHandler(t *testing.T) {
	tests := []struct {
		name       string
		records    []string
		lookupErr  error
		reject     bool
		wantStatus int
		wantBody   string
	}{
		{name: "successful lookup", records: []string{"64500|TEST"}, wantStatus: http.StatusOK, wantBody: "64500\n"},
		{name: "empty lookup", wantStatus: http.StatusOK, wantBody: "ERR\n"},
		{name: "lookup error", lookupErr: errors.New("DNS failed"), wantStatus: http.StatusOK, wantBody: "ERR\n"},
		{name: "rejected request", reject: true, wantStatus: http.StatusForbidden, wantBody: "forbidden\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{lookupTXT: func(query string) ([]string, error) {
				if query != "113.0.203.origin.asn.cymru.com" {
					t.Errorf("lookup query = %q", query)
				}
				return tt.records, tt.lookupErr
			}}
			r := trustedRequest(http.MethodGet, "/asn")
			if tt.reject {
				r.RemoteAddr = "192.0.2.1:12345"
			}
			w := httptest.NewRecorder()
			app.asnHandler(w, r)
			assertResponse(t, w, tt.wantStatus, tt.wantBody)
			if !tt.reject && w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Errorf("unexpected Content-Type %q", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestWriteJSONAllowsSimpleCrossOriginGETs(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"ip": "203.0.113.10"})
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Access-Control-Allow-Methods = %q, want it omitted", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "" {
		t.Errorf("Access-Control-Allow-Headers = %q, want it omitted", got)
	}
	assertJSONResponse(t, w, "ip", "203.0.113.10")
}

type failingResponseWriter struct{ header http.Header }

func (w *failingResponseWriter) Header() http.Header     { return w.header }
func (*failingResponseWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (*failingResponseWriter) WriteHeader(int)           {}

func TestWriteJSONLogsEncoderFailure(t *testing.T) {
	var output bytes.Buffer
	originalWriter := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(originalWriter) })

	writeJSON(&failingResponseWriter{header: make(http.Header)}, map[string]string{"ip": "203.0.113.10"})
	if !strings.Contains(output.String(), "write JSON response: write failed") {
		t.Fatalf("log output = %q", output.String())
	}
}

func TestIPAPIHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		w := httptest.NewRecorder()
		ipApiHandler(w, trustedRequest(http.MethodGet, "/api/ip"))
		assertJSONResponse(t, w, "ip", "203.0.113.10")
	})
	t.Run("rejected request", func(t *testing.T) {
		r := trustedRequest(http.MethodGet, "/api/ip")
		r.RemoteAddr = "192.0.2.1:12345"
		w := httptest.NewRecorder()
		ipApiHandler(w, r)
		assertResponse(t, w, http.StatusForbidden, "forbidden\n")
	})
}

func TestPTRAPIHandler(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		lookupErr error
		reject    bool
		want      string
	}{
		{name: "successful lookup", addresses: []string{"host.example."}, want: "host.example."},
		{name: "empty lookup", want: "none"},
		{name: "lookup error", lookupErr: errors.New("DNS failed"), want: "none"},
		{name: "rejected request", reject: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{lookupAddr: func(string) ([]string, error) { return tt.addresses, tt.lookupErr }}
			r := trustedRequest(http.MethodGet, "/api/ptr")
			if tt.reject {
				r.RemoteAddr = "192.0.2.1:12345"
			}
			w := httptest.NewRecorder()
			app.ptrApiHandler(w, r)
			if tt.reject {
				assertResponse(t, w, http.StatusForbidden, "forbidden\n")
				return
			}
			assertJSONResponse(t, w, "ptr", tt.want)
		})
	}
}

func TestASNAPIHandler(t *testing.T) {
	tests := []struct {
		name      string
		records   []string
		lookupErr error
		reject    bool
		want      string
	}{
		{name: "successful lookup", records: []string{"64500|TEST"}, want: "64500"},
		{name: "empty lookup", want: "ERR"},
		{name: "lookup error", lookupErr: errors.New("DNS failed"), want: "ERR"},
		{name: "rejected request", reject: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{lookupTXT: func(string) ([]string, error) { return tt.records, tt.lookupErr }}
			r := trustedRequest(http.MethodGet, "/api/asn")
			if tt.reject {
				r.RemoteAddr = "192.0.2.1:12345"
			}
			w := httptest.NewRecorder()
			app.asnApiHandler(w, r)
			if tt.reject {
				assertResponse(t, w, http.StatusForbidden, "forbidden\n")
				return
			}
			assertJSONResponse(t, w, "asn", tt.want)
		})
	}
}

func TestMuxRoutes(t *testing.T) {
	app := &application{
		lookupAddr: func(string) ([]string, error) { return []string{"host.example."}, nil },
		lookupTXT:  func(string) ([]string, error) { return []string{"64500|TEST"}, nil },
	}
	handler := newMux(app)

	for _, path := range []string{"/health", "/ip", "/ptr", "/asn", "/api/ip", "/api/ptr", "/api/asn"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, trustedRequest(http.MethodGet, path))
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", path, w.Code, http.StatusOK)
			}
		})
	}

	for _, path := range []string{"/health", "/ip"} {
		t.Run("unsupported method "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, trustedRequest(http.MethodPost, path))
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("POST %s status = %d, want %d", path, w.Code, http.StatusMethodNotAllowed)
			}
		})
	}

	t.Run("unknown path", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, trustedRequest(http.MethodGet, "/missing"))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestNewServer(t *testing.T) {
	handler := http.NewServeMux()
	server := newServer(handler)
	if server.Addr != ":3000" {
		t.Errorf("Addr = %q, want %q", server.Addr, ":3000")
	}
	if server.Handler != handler {
		t.Error("Handler does not match the configured handler")
	}
	if server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, 5*time.Second)
	}
}

func TestCheckHealth(t *testing.T) {
	t.Run("HTTP 200 succeeds", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok\n")),
			}, nil
		})}

		if err := checkHealth(client, "http://health.test/health"); err != nil {
			t.Fatalf("checkHealth() error = %v", err)
		}
	})

	t.Run("non-200 fails", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Status:     "503 Service Unavailable",
				Body:       io.NopCloser(strings.NewReader("unhealthy\n")),
			}, nil
		})}

		err := checkHealth(client, "http://health.test/health")
		if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
			t.Fatalf("checkHealth() error = %v, want HTTP 503 error", err)
		}
	})

	t.Run("request error fails", func(t *testing.T) {
		err := checkHealth(http.DefaultClient, "://invalid-url")
		if err == nil || !strings.Contains(err.Error(), "request failed") {
			t.Fatalf("checkHealth() error = %v, want request error", err)
		}
	})

	t.Run("timeout fails", func(t *testing.T) {
		client := &http.Client{
			Timeout: time.Millisecond,
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			}),
		}
		err := checkHealth(client, "http://health.test/health")
		if err == nil || !strings.Contains(err.Error(), "request failed") {
			t.Fatalf("checkHealth() error = %v, want timeout error", err)
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("healthcheck mode", func(t *testing.T) {
		var stderr bytes.Buffer
		healthChecked := false
		serverStarted := false

		exitCode := run([]string{"--healthcheck"}, &stderr, func() error {
			healthChecked = true
			return nil
		}, func() error {
			serverStarted = true
			return nil
		})

		if exitCode != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
		}
		if !healthChecked {
			t.Error("health check was not run")
		}
		if serverStarted {
			t.Error("server started in healthcheck mode")
		}
	})

	t.Run("default mode", func(t *testing.T) {
		var stderr bytes.Buffer
		healthChecked := false
		serverStarted := false

		exitCode := run(nil, &stderr, func() error {
			healthChecked = true
			return nil
		}, func() error {
			serverStarted = true
			return nil
		})

		if exitCode != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
		}
		if healthChecked {
			t.Error("health check ran in default mode")
		}
		if !serverStarted {
			t.Error("server was not started")
		}
	})

	t.Run("healthcheck failure", func(t *testing.T) {
		var stderr bytes.Buffer
		exitCode := run([]string{"--healthcheck"}, &stderr, func() error {
			return errors.New("connection refused")
		}, func() error {
			t.Fatal("server started in healthcheck mode")
			return nil
		})

		if exitCode != 1 {
			t.Errorf("exit code = %d, want 1", exitCode)
		}
		if got := stderr.String(); !strings.Contains(got, "health check failed: connection refused") {
			t.Errorf("stderr = %q, want health check error", got)
		}
	})
}
