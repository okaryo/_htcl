# Chunked Transfer

Chunked transfer encoding lets a response carry a body without knowing the
total length up front. Instead of `Content-Length`, the body is sent as repeated
chunks:

```text
5\r\n
hello\r\n
0\r\n
\r\n
```

Each chunk starts with a hexadecimal size line, followed by that many bytes of
data and a CRLF terminator. A zero-sized chunk ends the body. Trailer headers
can follow the final chunk.

## Current Boundary

The current implementation supports fixed in-memory decoding:

- `Transfer-Encoding: chunked` is accepted.
- Chunk extensions are ignored.
- Trailer headers are parsed and discarded.
- The decoded body is still stored as `Response.Body`.
- If `Content-Encoding: gzip` is also present, gzip decompression happens after
  chunk decoding.

Streaming chunk decoding is a later step.
