# Known Limitations

This project is a learning HTTP client, not a production replacement for
`net/http.Client`, `curl`, or `wget`.

The limitations below are intentional checkpoints. They keep the implementation
small enough to study while marking clear future learning directions.

## Protocol Scope

- The client implements HTTP/1.x request and response behavior only.
- HTTP/2 is not implemented.
- HTTP/3 and QUIC are not implemented.
- Upgrade flows such as WebSocket are not implemented.

## Response Parsing And Body Handling

- `ReadResponse` still returns `Body []byte`, so the parsed response body is
  buffered in memory.
- Fixed-length response streaming exists as a low-level helper, but the main
  response API does not expose a streaming body reader yet.
- Chunked response decoding currently buffers the decoded body.
- Gzip decoding currently buffers the decoded body.
- Only `gzip` and identity/no content encoding are supported. Encodings such as
  `br`, `deflate`, and `zstd` are rejected.
- Trailer fields from chunked responses are discarded.

## Request Body Handling

- Request bodies can be streamed only when the length is known up front.
- Request-side chunked transfer encoding is not implemented.
- The CLI accepts literal `-body` input, but it does not yet stream request
  bodies from files.
- Retrying requests with non-replayable streaming bodies is not fully modeled.

## Connection Management

- `Client.Do` is intentionally one-shot and sends `Connection: close`.
- `Client.DoReusable` keeps at most one idle connection per TCP address.
- There is no production-style connection pool with concurrency limits,
  per-host queues, or multiple idle connections.
- Connection reuse is tested for simple fixed-length responses, not every
  possible HTTP/1.1 framing combination.

## Redirects, Cookies, And Authentication

- Redirect handling is implemented for common `3xx` statuses, but it is not a
  full browser-compatible redirect policy.
- Cookie support is intentionally minimal. It covers parsing, storage, domain
  and path matching, and expiration, but it does not implement every browser
  cookie rule.
- Authentication helpers currently cover Basic authentication only.

## TLS And Proxy Support

- Direct HTTPS is supported through Go's `crypto/tls`.
- The client offers `http/1.1` through ALPN and does not negotiate HTTP/2.
- HTTPS through an HTTP proxy is not implemented because `CONNECT` tunneling is
  not implemented yet.
- Plain HTTP proxy requests are supported only for URL-based requests.

## Retry And Robustness

- Retry behavior is opt-in and intentionally conservative.
- Retries use broad error classification and idempotent method checks, but they
  do not yet respect `Retry-After`.
- Backoff has a cap but no jitter.
- Error classification is useful for learning and debugging, but it is not a
  complete production error taxonomy.

## CLI And Observability

- CLI output modes are limited to `response`, `body`, `headers`, and `status`.
- The CLI can save a parsed response body with `-save`, but it does not stream
  directly from the network connection into the output file yet.
- CLI progress output is not implemented.
- Structured debug events exist as a library hook, but the CLI does not expose
  a JSON log or trace format yet.

## RFC Completeness

Many RFC edge cases are intentionally out of scope for now, including complete
header grammar handling, every transfer-coding combination, every cache
directive, and browser-grade cookie behavior.

Those may be revisited later when they support a specific learning goal.
