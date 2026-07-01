# Robustness And Error Classification

This step starts separating different kinds of failures that used to be visible
only as formatted error strings.

## ClientError

Client operations can now return a `ClientError`:

```go
var clientErr *http1.ClientError
if errors.As(err, &clientErr) {
    fmt.Println(clientErr.Kind)
    fmt.Println(clientErr.Phase)
}
```

`Kind` answers what kind of failure happened:

- `network`: dialing, writing, or reading failed at the transport layer
- `timeout`: a context deadline or connection deadline expired
- `protocol`: the HTTP message was invalid or could not be serialized
- `application`: the server returned an HTTP status that the caller chooses to
  treat as an application-level error

`Phase` answers where the failure happened:

- `dial`
- `tls_handshake`
- `serialize_request`
- `write_request`
- `read_response`
- `response_status`

The original error is still wrapped, so `errors.Is` and `errors.As` can inspect
lower-level errors such as `context.DeadlineExceeded`, `net.Error`, or TLS
certificate verification errors.

## Response Status Is Separate

The client still returns `4xx` and `5xx` responses as successful HTTP responses.
That matches common HTTP client behavior: receiving a valid `HTTP/1.1 404 Not
Found` response means the HTTP exchange succeeded, even if the application-level
result is a failure.

Callers that want to treat those statuses as errors can use:

```go
err := http1.ResponseStatusError(response)
```

That returns an `application` `ClientError` for status codes `400` and above.

## Current Limits

This classification is intentionally small. It does not yet decide whether an
error is retryable, whether a request body was partially written, or whether a
streaming body can be replayed. Those decisions need more phase detail and are
left for the retry refinement step.
