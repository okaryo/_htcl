# Client API And CLI Shape

The project currently has two layers:

- `internal/http1` contains the learning HTTP client pieces.
- `cmd/htcl` turns command-line flags into an HTTP request and prints the parsed
  response.

## Current Client API

The current client API has two main request paths:

- `Client.Do`: one-shot request path. It sends `Connection: close`, performs one
  round trip, and closes the TCP connection.
- `Client.DoReusable`: minimal keep-alive path. It keeps at most one idle
  connection per TCP address.

Both paths currently accept a TCP address and a prepared `Request`. URL parsing,
method selection, and output formatting still live in the CLI.

## Current CLI

The command can make a request from either a URL:

```sh
go run ./cmd/htcl -method GET http://127.0.0.1:8080/hello
```

or lower-level pieces:

```sh
go run ./cmd/htcl -addr 127.0.0.1:8080 -host localhost -target /hello
```

Supported CLI options currently include:

- `-method`: HTTP method. Defaults to `GET`.
- `-header`: HTTP request header in `Name: value` form. Can be repeated.
- `-body`: HTTP request body as a literal string.
- `-basic`: Basic authentication credentials in `user:password` form.
- `-no-cache`: send `Cache-Control: no-cache`.
- `-if-none-match`: send `If-None-Match` with the given ETag.
- `-if-modified-since`: send `If-Modified-Since` from an RFC3339 timestamp.
- `-follow`: follow redirects.
- `-max-redirects`: maximum number of redirects to follow. Defaults to `10`.
- `-proxy`: HTTP proxy URL for URL-based requests.
- `-retries`: maximum retry attempts for idempotent URL-based requests.
- `-insecure`: skip TLS certificate verification for local HTTPS experiments.
- `-url`: URL to request. A positional URL is also accepted.
- `-addr`: TCP address for lower-level observation.
- `-host`: HTTP `Host` header for lower-level observation.
- `-target`: HTTP request target for lower-level observation.
- `-timeout`: dial, write, and read timeout.
- `-output`: response output mode. Supported values are `response`, `body`,
  `headers`, and `status`.

The CLI starts with default `Host` and `User-Agent` headers, then applies
`-header` values. Repeated names replace earlier values, so custom `Host` or
`User-Agent` values are visible in the serialized request.

When `-body` is non-empty, the request model writes the bytes after the blank
line and automatically adds `Content-Length` unless the caller provided a
matching value.

When `-proxy` is set, the CLI connects to the proxy address but keeps the
origin `Host` header. The request target is serialized in absolute-form, for
example `GET http://example.test/path HTTP/1.1`. HTTPS proxy tunneling with
`CONNECT` is not implemented yet.

When `-retries` is greater than zero, failed URL-based requests may be retried
only for idempotent methods such as `GET`, `HEAD`, `PUT`, and `DELETE`.
Non-idempotent methods such as `POST` are not retried automatically.

When the URL uses `https://`, the CLI connects with TLS before writing the same
HTTP/1.1 request. Certificate verification uses Go's standard TLS behavior
unless `-insecure` is set. Successful HTTPS requests also print a small TLS
summary to stderr, including the negotiated version and cipher suite.

Output modes are handled only by the CLI. The client package still returns a
parsed `Response`; the command chooses which parts to print.
