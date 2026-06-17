# HTTP Response Parsing

`ReadResponse` combines the first response parsing steps into one minimal
HTTP/1.x response parser.

The current sequence is:

```text
LineReader.ReadLine
  -> ParseStatusLine
  -> ReadHeaderFields
  -> reject unsupported Transfer-Encoding
  -> ContentLength
  -> ReadFixedBody
```

The parser still reads from the same TCP connection, but the command no longer
treats response bytes as an opaque stream. It now separates the status line,
headers, and fixed-length body.

## Current Response Model

The parsed response contains:

- The HTTP version.
- The numeric status code.
- The reason phrase.
- Parsed header fields.
- The fixed-length body, if `Content-Length` is present.

The parser preserves header order and original field-name casing.

## Current Validation

The parser currently rejects:

- Incomplete CRLF-terminated lines.
- Lines that do not end with CRLF.
- Unsupported HTTP versions.
- Malformed status codes.
- Malformed header fields.
- Invalid or conflicting `Content-Length` values.
- Unsupported `Transfer-Encoding` values.
- Bodies that end before the declared `Content-Length`.

The parser currently treats responses without `Content-Length` as having no
body. Close-delimited bodies and chunked response bodies are intentionally left
for later learning steps.
