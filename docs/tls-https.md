# TLS And HTTPS

This step adds the first HTTPS behavior: wrapping the TCP connection with TLS
before writing the HTTP/1.1 request.

## HTTP Versus HTTPS

For `http://` URLs, the client writes HTTP bytes directly to a TCP connection:

```text
TCP connect
HTTP request
HTTP response
TCP close
```

For `https://` URLs, the client first opens TCP, then performs a TLS handshake,
then sends the same HTTP/1.1 request bytes inside the encrypted TLS connection:

```text
TCP connect
TLS handshake
HTTP request over TLS
HTTP response over TLS
TLS/TCP close
```

The request line still uses origin-form for a direct HTTPS request:

```http
GET /path?q=1 HTTP/1.1
Host: example.test
```

TLS changes the connection, not the HTTP request model.

## Server Name

The client passes the URL host name as the TLS server name. That name is used by
the TLS stack for certificate verification and for Server Name Indication.

The `Host` header may include a port, but the TLS server name is just the host
name:

```text
URL: https://example.test:8443/path
TLS server name: example.test
Host header: example.test:8443
TCP address: example.test:8443
```

## Current CLI Behavior

The CLI now accepts direct `https://` URLs:

```sh
go run ./cmd/htcl https://example.test/
```

By default, certificate verification is handled by Go's standard `crypto/tls`
behavior.

For local learning servers with self-signed certificates, the CLI also has
`-insecure`:

```sh
go run ./cmd/htcl -insecure https://127.0.0.1:8443/
```

This disables certificate verification and should be used only for local
experiments.

## What Is Still Missing

HTTPS through an HTTP proxy is not implemented yet. That requires `CONNECT` to
create a tunnel through the proxy before starting the TLS handshake.

More detailed TLS topics are still future steps:

- observing handshake state
- certificate verification behavior
- Server Name Indication details
- ALPN and HTTP/2 negotiation
