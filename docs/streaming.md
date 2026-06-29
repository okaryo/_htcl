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

## Progress Reporting

For fixed-length bodies, progress can be reported because the total size is
known from `Content-Length`:

```go
written, err := StreamFixedBodyWithProgress(dst, conn, length, func(p Progress) {
    fmt.Printf("%d/%d\n", p.Written, p.Total)
})
```

The progress callback reports bytes that were successfully written to the
destination. This makes it suitable for file downloads, hashes, or other
streaming destinations.

This is still a low-level building block. The CLI does not yet render a progress
bar; it can use this hook once response bodies are streamed directly from the
network connection to the output file.

## Current Limitations

This is only the first streaming building block:

- `ReadResponse` still buffers the final decoded body into memory.
- Chunked transfer streaming is not separated yet.
- Gzip decoding still produces a full decoded `[]byte`.
- The CLI can save the parsed response body with `-save`, but it does not yet
  stream directly from the network connection to the file.
- CLI progress output is not implemented yet.

Those are later steps. The important boundary introduced here is that body
framing can be copied to an `io.Writer` without requiring a full in-memory
buffer.

## Current File Output

The CLI can save the response body:

```sh
go run ./cmd/htcl -save body.bin -output status http://127.0.0.1:8080/file
```

`-save` writes only the response body to the file. `-output` still controls what
is printed to stdout.
