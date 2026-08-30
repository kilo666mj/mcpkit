// Package mcpkit provides small, opinionated helpers shared by Go MCP servers.
//
// It standardizes server metadata, tool safety annotations, local stdio
// lifecycle, and bounded stateless HTTP transport. Applications retain their
// tool schemas, authentication, authorization, auditing, and domain behavior.
package mcpkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const DefaultMaxRequestBodyBytes int64 = 1 << 20

// ServerConfig is the metadata and behavior shared by one application's MCP
// server. Name is required. Version should normally be the application build
// version rather than a separate MCP API version.
type ServerConfig struct {
	Name         string
	Version      string
	Instructions string
	Logger       *slog.Logger
	PageSize     int
}

// NewServer constructs an official-SDK server with consistent metadata.
func NewServer(cfg ServerConfig) (*mcp.Server, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, errors.New("MCP server name is required")
	}
	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = "dev"
	}
	if cfg.PageSize < 0 {
		return nil, errors.New("MCP page size cannot be negative")
	}
	return mcp.NewServer(
		&mcp.Implementation{Name: name, Version: version},
		&mcp.ServerOptions{Instructions: cfg.Instructions, Logger: cfg.Logger, PageSize: cfg.PageSize},
	), nil
}

// MustServer is NewServer for application initialization where invalid static
// configuration is a programming error.
func MustServer(cfg ServerConfig) *mcp.Server {
	server, err := NewServer(cfg)
	if err != nil {
		panic(err)
	}
	return server
}

// ReadOnly marks a tool as side-effect free and idempotent. Set openWorld when
// it may read from arbitrary external entities rather than a closed service or
// configured data set. MCP annotations are advisory client hints; applications
// must still enforce authorization and safety policy in their handlers.
func ReadOnly(openWorld bool) *mcp.ToolAnnotations {
	return annotations(true, false, true, openWorld)
}

// Mutating marks an additive or non-destructive write. Idempotent describes
// whether repeating the same call has no additional effect.
func Mutating(idempotent, openWorld bool) *mcp.ToolAnnotations {
	return annotations(false, false, idempotent, openWorld)
}

// Destructive marks a write that may overwrite, revoke, or delete state.
func Destructive(idempotent, openWorld bool) *mcp.ToolAnnotations {
	return annotations(false, true, idempotent, openWorld)
}

func annotations(readOnly, destructive, idempotent, openWorld bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint: readOnly, DestructiveHint: boolPointer(destructive),
		IdempotentHint: idempotent, OpenWorldHint: boolPointer(openWorld),
	}
}

func boolPointer(value bool) *bool { return &value }

// RunStdio serves until the client disconnects or ctx is cancelled. Normal
// transport closure is reported as success so command entry points do not need
// to duplicate SDK-specific EOF handling.
func RunStdio(ctx context.Context, server *mcp.Server) error {
	if server == nil {
		return errors.New("MCP server is required")
	}
	err := server.Run(ctx, &mcp.StdioTransport{})
	if NormalClose(err) {
		return nil
	}
	return err
}

// NormalClose reports errors produced by an expected client disconnect or
// caller cancellation. Wrapped errors must preserve their cause for errors.Is;
// message text is deliberately not used to classify process exit status.
func NormalClose(err error) bool {
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, context.Canceled)
}

// HTTPOptions controls the bounded stateless Streamable HTTP helper.
type HTTPOptions struct {
	// MaxRequestBodyBytes defaults to 1 MiB. Negative values are invalid; zero
	// selects the default rather than disabling the limit.
	MaxRequestBodyBytes int64
	Logger              *slog.Logger
	// TrustedOrigins permits exact browser Origin values such as
	// "https://console.example.com" while retaining protection against all
	// other cross-origin browser requests.
	TrustedOrigins []string
	// DisableBrowserOriginProtection disables only Go's outer Origin and
	// Sec-Fetch-Site checks. It does not disable the SDK's independent localhost
	// DNS-rebinding protection and it is not an authentication mechanism. Use it
	// only when a trusted outer HTTP layer already enforces browser origins.
	DisableBrowserOriginProtection bool
	// DisableLocalhostProtection permits reverse proxies that connect to a
	// loopback listener while preserving the external Host header. Use it only
	// when a trusted proxy or network boundary prevents direct untrusted access
	// to that listener. Browser origin protection remains independent.
	DisableLocalhostProtection bool
}

// StatelessHTTP returns a JSON-response Streamable HTTP handler with a request
// body limit and browser cross-origin protection.
//
// The returned handler is publicly callable unless the application wraps it in
// authentication and authorization middleware. Cross-origin and localhost
// protections defend browser and DNS-rebinding boundaries; they are not access
// control and non-browser clients may send neither relevant header. Public
// deployments must also provide appropriate rate, concurrency, and request
// timeout limits. Authentication should wrap this handler so the factory can
// derive identity from r.Context; returning nil for a missing identity makes
// the SDK reject the request.
//
// SDK logging can include tool arguments at debug level. Do not attach a debug
// logger in production when tool inputs may contain sensitive information.
func StatelessHTTP(factory func(*http.Request) *mcp.Server, opts HTTPOptions) (http.Handler, error) {
	if factory == nil {
		return nil, errors.New("MCP server factory is required")
	}
	limit := opts.MaxRequestBodyBytes
	if limit < 0 {
		return nil, fmt.Errorf("MCP maximum request body bytes cannot be negative")
	}
	if limit == 0 {
		limit = DefaultMaxRequestBodyBytes
	}
	var handler http.Handler = mcp.NewStreamableHTTPHandler(factory, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: limit, Logger: opts.Logger,
		DisableLocalhostProtection: opts.DisableLocalhostProtection,
	})
	if !opts.DisableBrowserOriginProtection {
		protection := http.NewCrossOriginProtection()
		for _, origin := range opts.TrustedOrigins {
			if err := protection.AddTrustedOrigin(strings.TrimSpace(origin)); err != nil {
				return nil, fmt.Errorf("trusted MCP origin %q: %w", origin, err)
			}
		}
		handler = protection.Handler(handler)
	}
	return handler, nil
}
