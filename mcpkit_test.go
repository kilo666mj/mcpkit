package mcpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewServerValidation(t *testing.T) {
	if _, err := NewServer(ServerConfig{}); err == nil {
		t.Fatal("empty name accepted")
	}
	if _, err := NewServer(ServerConfig{Name: "service", PageSize: -1}); err == nil {
		t.Fatal("negative page size accepted")
	}
	if _, err := NewServer(ServerConfig{Name: "service"}); err != nil {
		t.Fatal(err)
	}
}

func TestSafetyAnnotations(t *testing.T) {
	read := ReadOnly(false)
	if !read.ReadOnlyHint || !read.IdempotentHint || *read.DestructiveHint || *read.OpenWorldHint {
		t.Fatalf("read annotations = %+v", read)
	}
	write := Mutating(true, true)
	if write.ReadOnlyHint || *write.DestructiveHint || !write.IdempotentHint || !*write.OpenWorldHint {
		t.Fatalf("mutating annotations = %+v", write)
	}
	remove := Destructive(false, false)
	if remove.ReadOnlyHint || !*remove.DestructiveHint || remove.IdempotentHint || *remove.OpenWorldHint {
		t.Fatalf("destructive annotations = %+v", remove)
	}
}

func TestNormalClose(t *testing.T) {
	for _, err := range []error{nil, io.EOF, context.Canceled, fmt.Errorf("transport: %w", io.EOF)} {
		if !NormalClose(err) {
			t.Errorf("NormalClose(%v) = false", err)
		}
	}
	for _, err := range []error{errors.New("connection failed"), errors.New("transport: EOF")} {
		if NormalClose(err) {
			t.Errorf("unexpected failure %q treated as normal close", err)
		}
	}
}

func TestStatelessHTTP(t *testing.T) {
	server := MustServer(ServerConfig{Name: "test", Version: "1"})
	handler, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, HTTPOptions{MaxRequestBodyBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"`+strings.Repeat("x", 33)+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestStatelessHTTPRejectsCrossOrigin(t *testing.T) {
	server := MustServer(ServerConfig{Name: "test"})
	handler, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://service.example/mcp", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestStatelessHTTPBrowserOriginOptions(t *testing.T) {
	server := MustServer(ServerConfig{Name: "test"})
	for _, test := range []struct {
		name    string
		opts    HTTPOptions
		origin  string
		blocked bool
	}{
		{name: "untrusted", origin: "https://attacker.example", blocked: true},
		{name: "trusted", opts: HTTPOptions{TrustedOrigins: []string{"https://console.example"}}, origin: "https://console.example"},
		{name: "disabled", opts: HTTPOptions{DisableBrowserOriginProtection: true}, origin: "https://attacker.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, test.opts)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "https://service.example/mcp", strings.NewReader(`{}`))
			request.Header.Set("Origin", test.origin)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if got := recorder.Code == http.StatusForbidden; got != test.blocked {
				t.Fatalf("status = %d, blocked=%t", recorder.Code, got)
			}
		})
	}
	if _, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, HTTPOptions{TrustedOrigins: []string{"not an origin"}}); err == nil {
		t.Fatal("invalid trusted origin accepted")
	}
}

func TestStatelessHTTPDefaultBodyLimit(t *testing.T) {
	server := MustServer(ServerConfig{Name: "test"})
	handler, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"` + strings.Repeat("x", int(DefaultMaxRequestBodyBytes)) + `"}`
	request := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestStatelessHTTPLocalhostProtectionOption(t *testing.T) {
	server := MustServer(ServerConfig{Name: "test"})
	for _, test := range []struct {
		name       string
		disable    bool
		wantStatus int
	}{
		{name: "protected", wantStatus: http.StatusForbidden},
		{name: "trusted proxy", disable: true, wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, err := StatelessHTTP(func(*http.Request) *mcp.Server { return server }, HTTPOptions{DisableLocalhostProtection: test.disable})
			if err != nil {
				t.Fatal(err)
			}
			httpServer := httptest.NewServer(handler)
			defer httpServer.Close()
			request, err := http.NewRequest(http.MethodPost, httpServer.URL, strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Host = "mcp.example.com"
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			response, err := httpServer.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}
