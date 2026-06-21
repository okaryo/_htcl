# URL Handling

The client can now derive connection and request details from a URL.

The current sequence is:

```text
ParseURL
  -> validate scheme and authority
  -> TCPAddressForURL
  -> HostHeaderForURL
  -> RequestTargetForURL
  -> NewRequestForURL
```

## Current URL Model

The URL helper uses Go's `net/url` parser and then makes the client-specific
decisions explicit:

- `http` defaults to TCP port `80`.
- `https` defaults to TCP port `443`, but the command does not connect to HTTPS
  yet because TLS support is a later step.
- The `Host` header comes from the URL authority.
- The TCP address uses the URL hostname plus explicit or default port.
- The request target uses the escaped path and raw query.
- Fragments are not sent in the HTTP request target.
- Missing paths are sent as `/`.

## Current CLI Shape

The command accepts a URL as a positional argument:

```sh
go run ./cmd/htcl http://127.0.0.1:8080/hello?name=htcl
```

The older `-addr`, `-host`, and `-target` flags still work for observing the
lower-level pieces independently.

## Current Validation

The URL helper currently rejects:

- URLs without a scheme.
- Unsupported schemes.
- URLs without a host.
- URLs with user info.
- URLs whose hostname cannot be determined.

`https` URLs are parsed, but the command returns an explicit error until TLS
connection support is implemented.
