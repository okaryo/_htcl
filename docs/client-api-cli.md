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
- `-url`: URL to request. A positional URL is also accepted.
- `-addr`: TCP address for lower-level observation.
- `-host`: HTTP `Host` header for lower-level observation.
- `-target`: HTTP request target for lower-level observation.
- `-timeout`: dial, write, and read timeout.

The CLI starts with default `Host` and `User-Agent` headers, then applies
`-header` values. Repeated names replace earlier values, so custom `Host` or
`User-Agent` values are visible in the serialized request.

Request bodies and output modes are later steps.
