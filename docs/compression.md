# Compression

HTTP response compression is indicated by the `Content-Encoding` response
header. It is different from `Transfer-Encoding`: transfer encoding describes
how bytes are framed on the wire, while content encoding describes how the
representation body is encoded.

## Current Boundary

The current implementation supports gzip response bodies:

- The response parser reads either a fixed-length body using `Content-Length` or
  a chunked body using `Transfer-Encoding: chunked`.
- If `Content-Encoding: gzip` is present, the body is decompressed after the
  compressed bytes have been read and any transfer framing has been removed.
- `Content-Length` remains the length of the compressed wire body, not the
  decompressed body.
- Unsupported content encodings return an error.

The client does not automatically send `Accept-Encoding` yet. Streaming
decompression is a later step.
