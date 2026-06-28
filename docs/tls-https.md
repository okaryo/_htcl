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

SNI is sent in the TLS `ClientHello` before the HTTP request exists. This lets a
server choose the right certificate when many host names share the same IP
address and port.

For example, a client may connect to `127.0.0.1:8443` while still sending
`example.test` as the TLS server name:

```text
TCP address: 127.0.0.1:8443
TLS SNI: example.test
Host header: example.test
```

These values often match in normal browsing, but they are different layers:

- TCP address decides where the socket connects.
- TLS SNI tells the TLS server which host name the client wants.
- HTTP `Host` tells the HTTP server which origin the request targets.

## Current CLI Behavior

The CLI now accepts direct `https://` URLs:

```sh
go run ./cmd/htcl https://example.test/
```

By default, certificate verification is handled by Go's standard `crypto/tls`
behavior.

That verification happens during `HandshakeContext`, before the HTTP request is
written. At this stage Go checks the server certificate using the configured
root certificate pool and the TLS server name:

- the certificate must chain to a trusted root
- the certificate must be valid for the requested server name
- the certificate must be within its validity period

If verification fails, the client returns an error and does not write the HTTP
request.

After a successful handshake, the CLI prints a small TLS summary to stderr:

```text
tls version: TLS 1.3
tls cipher suite: TLS_AES_128_GCM_SHA256
tls server name: example.test
tls peer certificates: 2
tls verified chains: 1
```

This is not a packet-level trace. It is the negotiated connection state reported
by Go's `tls.Conn.ConnectionState()` after `HandshakeContext` completes.

For local learning servers with self-signed certificates, the CLI also has
`-insecure`:

```sh
go run ./cmd/htcl -insecure https://127.0.0.1:8443/
```

This disables certificate verification and should be used only for local
experiments. It means the client still encrypts the connection, but it no
longer verifies that the peer is really the requested server.

## What Is Still Missing

HTTPS through an HTTP proxy is not implemented yet. That requires `CONNECT` to
create a tunnel through the proxy before starting the TLS handshake.

More detailed TLS topics are still future steps:

- ALPN and HTTP/2 negotiation
