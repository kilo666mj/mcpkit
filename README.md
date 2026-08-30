# mcpkit

`mcpkit` is a small, opinionated helper package for Go applications that expose
Model Context Protocol servers with the official Go SDK.

It standardizes the pieces that should be consistent across applications:

- server identity, instructions, and logging;
- explicit read-only, mutating, and destructive tool annotations;
- clean stdio shutdown;
- bounded stateless Streamable HTTP with cross-origin protection;
- initialized in-memory client sessions for black-box tests.

Applications still own tool schemas, authentication, authorization, audit
identity, confirmation and idempotency policy, and domain behavior.

## Server and tools

```go
server := mcpkit.MustServer(mcpkit.ServerConfig{
    Name:         "inventory",
    Version:      version,
    Instructions: "Use inventory tools for live infrastructure facts.",
    Logger:       logger,
})

mcp.AddTool(server, &mcp.Tool{
    Name:        "inventory_list_hosts",
    Description: "List current managed hosts.",
    Annotations: mcpkit.ReadOnly(false),
}, listHosts)

if err := mcpkit.RunStdio(ctx, server); err != nil {
    return err
}
```

`openWorld` is explicit on every annotation helper. Use `false` for a closed
application or configured data set and `true` when a tool may interact with
arbitrary external entities. These annotations are advisory client hints, not
authorization or confirmation enforcement; handlers must enforce their own
safety policy.

## Stateless HTTP

Authentication wraps the MCP handler and supplies identity through the request
context. The application can then create a server whose tools close over that
identity.

```go
handler, err := mcpkit.StatelessHTTP(
    func(r *http.Request) *mcp.Server {
        return newServer(currentUser(r))
    },
    mcpkit.HTTPOptions{Logger: logger, MaxRequestBodyBytes: 2 << 20},
)
if err != nil {
    return err
}
mux.Handle("/mcp", requireBearer(handler))
```

The helper defaults to JSON responses, stateless sessions, a 1 MiB body limit,
SDK localhost protection, and Go browser cross-origin protection. An MCP
handler without an authentication wrapper is publicly callable: origin and
localhost checks are not access control, and non-browser clients commonly send
neither browser header. Authentication, authorization, Host allowlisting,
timeouts, rate limiting, and concurrency limits remain application and
deployment responsibilities.

Reverse proxies that connect over loopback while preserving an external Host
must set `DisableLocalhostProtection` only when the loopback listener cannot be
reached by untrusted clients. Prefer `TrustedOrigins` for legitimate browser
origins; `DisableBrowserOriginProtection` is reserved for an outer layer that
already enforces Origin and Sec-Fetch-Site.

The SDK logger can include tool arguments at debug level. Do not enable debug
logging in production when tool inputs may contain sensitive information.

## Tests

```go
session := mcpkittest.Connect(t, server)
result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
    Name: "inventory_list_hosts",
})
```

## Compatibility

`mcpkit` currently targets Go 1.26 and
`github.com/modelcontextprotocol/go-sdk` v1.7.0.

## License

MIT
