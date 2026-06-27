# Proxy Support

This step adds the first HTTP proxy behavior: sending an HTTP request through a
plain HTTP proxy.

## Direct HTTP Request

When the client connects directly to the origin server, the TCP connection goes
to the origin address and the request line uses origin-form:

```http
GET /search?q=hello HTTP/1.1
Host: example.test
```

In this form, the request target is only the path and query. The TCP connection
itself already tells the client which server it is talking to.

## HTTP Proxy Request

When the client uses an HTTP proxy, the TCP connection goes to the proxy
address, not the origin address. The request line uses absolute-form so the
proxy can see the destination URL:

```http
GET http://example.test/search?q=hello HTTP/1.1
Host: example.test
```

The `Host` header still names the origin server. The proxy address is only the
next TCP hop.

## Current CLI Behavior

The CLI accepts `-proxy` for URL-based requests:

```sh
go run ./cmd/htcl -proxy http://127.0.0.1:8080 http://example.test/search?q=hello
```

With this flag:

- TCP connects to the proxy URL host and port.
- The request target becomes the absolute URL.
- The `Host` header remains the origin host.
- Redirected requests keep using the same proxy.

## What Is Still Missing

This step does not implement `CONNECT`. HTTPS through a proxy normally starts
with a request like:

```http
CONNECT example.test:443 HTTP/1.1
Host: example.test:443
```

After the proxy opens the TCP tunnel, the client performs the TLS handshake
through that tunnel. That belongs with the later TLS/HTTPS learning steps.
