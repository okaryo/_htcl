# HTTP Request Serialization

The client now has a small request model and a serializer instead of building
the request with one `fmt.Sprintf` call inside the command.

The current sequence is:

```text
NewRequest or NewRequestForURL
  -> validate method, target, version, and headers
  -> ensure HTTP/1.1 has Host
  -> add or validate Content-Length
  -> WriteRequest
  -> write request line, headers, empty line, and fixed body bytes
```

## Current Request Model

The request model contains:

- The HTTP method.
- The request target.
- The HTTP version.
- Ordered header fields.
- An optional fixed byte slice body.

The serializer preserves header order and writes CRLF line endings.

## Host And Target

`NewRequest` expects the caller to provide the request target and required
headers explicitly.

`NewRequestForURL` accepts a parsed URL and derives:

- `Host` from `URL.Host`.
- The request target from the escaped path and raw query.

Full URL handling is still a later step. The current helper uses Go's `net/url`
model so the request serializer can focus on the HTTP bytes that go on the
wire.

## Current Validation

The serializer currently rejects:

- Empty methods.
- Methods containing whitespace.
- Targets that do not start with `/`.
- Targets containing line breaks.
- Unsupported HTTP versions.
- Header field names that are empty or contain invalid whitespace, CRLF, or
  colon characters.
- Header field values containing line breaks.
- HTTP/1.1 requests without a `Host` header.
- Invalid or conflicting `Content-Length` values.
- `Content-Length` values that do not match the fixed body length.

If a fixed body is present and `Content-Length` is missing, the serializer adds
the correct `Content-Length` header.
