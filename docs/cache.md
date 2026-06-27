# Cache Request Headers

This project does not implement an HTTP cache yet. The current step only
explores request headers that a client can send when it wants cached data to be
revalidated.

## Current Boundary

The current implementation provides helpers and CLI flags for:

- `Cache-Control: no-cache`: ask caches to revalidate before using a stored
  response.
- `If-None-Match`: send a previously received ETag and allow the server to
  return `304 Not Modified`.
- `If-Modified-Since`: send a previously received modification time and allow
  the server to return `304 Not Modified`.

The CLI accepts:

```sh
go run ./cmd/htcl -no-cache -if-none-match '"abc123"' http://127.0.0.1:8080/data
```

`-if-modified-since` accepts an RFC3339 timestamp and serializes it as an HTTP
date.

Storing responses, interpreting `Cache-Control` response directives, and
deciding when to reuse cached bodies are later steps.
