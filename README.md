# _htcl

`_htcl` is a learning-oriented HTTP client implementation in Go.

The goal of this project is not to build a production-ready replacement for
`curl` or Go's `net/http.Client`. The goal is to understand what sits underneath
the HTTP clients we usually use: URL handling, TCP connections, request
serialization, response parsing, body framing, timeouts, redirects, cookies,
connection reuse, TLS, and streaming.

## Purpose

This project is for studying HTTP client internals step by step.

The intended learning style is:

- Build small pieces incrementally.
- Confirm the learning objective before each major step.
- Prefer understanding the mechanism over quickly adding features.
- Compare behavior with Go's standard library and common tools when useful.
- Keep the roadmap flexible as new questions and interests appear.

The project assumes that the learner is already comfortable with Go and backend
development. Therefore, the focus is not on basic Go syntax or ordinary CLI
application structure, but on deeper implementation details.

## Learning Topics

This project may cover topics such as:

- TCP client basics: `dial`, `write`, `read`, blocking I/O, deadlines, and
  connection lifecycle.
- HTTP request structure: method, request target, version, headers, host,
  request bodies, and content framing.
- HTTP response structure: status line, headers, fixed-length bodies, chunked
  transfer, trailers, and malformed response handling.
- URL handling: scheme, host, port, path, query, fragments, percent-encoding,
  and authority parsing.
- Connection management: keep-alive, connection reuse, idle timeouts,
  connection pools, and cancellation.
- Client behavior: redirects, cookies, compression, cache-related headers,
  authentication headers, proxies, and retries.
- TLS and HTTPS: TLS handshake, certificate verification, ALPN, and how HTTPS
  changes the connection lifecycle.
- Streaming: large downloads, file output, streaming uploads, progress
  reporting, and backpressure.
- Observability and robustness: logging, tracing, error classification,
  timeout behavior, and resource cleanup.

## Non-goals

The following are not the main focus of this project:

- Building a full production-ready HTTP client.
- Replacing `curl`, `wget`, or Go's `net/http.Client`.
- Building a feature-complete CLI tool before understanding the protocol
  behavior.
- Prioritizing convenience flags over implementation understanding.

Some production-oriented topics may still be explored when they help explain how
real HTTP clients behave.

## Approach

The preferred starting point is the lower layer:

1. Start with `net.Dial` and a raw TCP connection.
2. Write a minimal HTTP/1.1 request by hand.
3. Read raw response bytes from the connection.
4. Parse the status line and headers.
5. Read fixed-length response bodies.
6. Add basic timeout and cancellation behavior.
7. Introduce a small request/response model.
8. Explore keep-alive, redirects, cookies, compression, TLS, and streaming.

At each stage, the implementation should remain small enough to inspect and
explain. When the design becomes unclear, the roadmap should be updated rather
than treated as fixed.

## Running the Current Client

The current implementation is a minimal HTTP/1.1 client.

Run the command against a local HTTP server:

```sh
go run ./cmd/htcl -method GET -header 'Accept: text/plain' http://127.0.0.1:8080/hello
```

Send a small request body:

```sh
go run ./cmd/htcl -method POST -body 'hello' http://127.0.0.1:8080/submit
```

Print only the response body:

```sh
go run ./cmd/htcl -output body http://127.0.0.1:8080/hello
```

The command opens a TCP connection, writes a manual HTTP/1.1 request, parses the
response status line, headers, and fixed-length body, then prints the parsed
response.

The default timeout is 30 seconds. To make blocking behavior easier to observe:

```sh
go run ./cmd/htcl -timeout 5s http://127.0.0.1:8080/hello
```

This first client sends `Connection: close`, but fixed-length response bodies
are now read using `Content-Length`. Keep-alive reuse and chunked transfer are
later learning steps.

## Project Documents

- `README.md`: project purpose, scope, and high-level learning direction.
- `AGENTS.md`: working instructions for AI agents and future contributors.
- `TODO.md`: living learning roadmap and progress tracker.
- `docs/tcp-http-get.md`: notes on the first raw TCP HTTP GET step.
- `docs/http-request.md`: notes on the first HTTP request serialization step.
- `docs/http-response.md`: notes on the first HTTP response parsing step.
- `docs/url-handling.md`: notes on URL-derived addresses, host headers, and
  request targets.
- `docs/connection-management.md`: notes on connection close and reuse
  decisions.
- `docs/client-api-cli.md`: notes on the current client API and command-line
  shape.
- `docs/redirects.md`: notes on redirect response detection and future follow
  behavior.
