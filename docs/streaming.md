# Streaming And Large Bodies

This step starts separating response body reading from response body storage.

The existing `ReadResponse` API still returns:

```go
Body []byte
```

That is useful for small learning examples, but it means the whole response body
must fit in memory.

## Fixed-Length Streaming

For responses with `Content-Length`, the client can stream exactly that many
bytes from the connection into an `io.Writer`:

```go
written, err := StreamFixedBody(dst, conn, length)
```

This is different from `ReadFixedBody`:

- `ReadFixedBody` returns `[]byte`, so it stores the entire body in memory.
- `StreamFixedBody` copies bytes to a writer, so the caller can stream to a
  file, hash, buffer, or another destination.

The helper copies exactly `Content-Length` bytes. It does not read beyond the
declared body, which matters when a keep-alive connection may contain the next
response after the current body.

## Current Limitations

This is only the first streaming building block:

- `ReadResponse` still buffers the final decoded body into memory.
- Chunked transfer streaming is not separated yet.
- Gzip decoding still produces a full decoded `[]byte`.
- The CLI does not yet stream directly to a file.

Those are later steps. The important boundary introduced here is that body
framing can be copied to an `io.Writer` without requiring a full in-memory
buffer.
