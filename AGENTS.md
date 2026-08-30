# AGENTS.md

Keep mcpkit small and application-neutral. Do not add domain tool schemas,
authentication policy, credential storage, or persistence behavior.

Use the official MCP Go SDK. Preserve secure defaults for HTTP request limits,
localhost protection, and cross-origin protection.

After Go changes, run:

```sh
gofmt -w .
go test ./...
go vet ./...
```
