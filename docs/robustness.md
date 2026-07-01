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

## Structured Debug Events

`Client` can emit structured debug events through `DebugLog`:

```go
client := http1.Client{
    DebugLog: func(event http1.DebugEvent) {
        fmt.Printf("%s %s %s\n", event.Name, event.Phase, event.Address)
    },
}
```

The hook receives events such as:

- `dial_start`
- `dial_done`
- `tls_handshake_start`
- `tls_handshake_done`
- `write_request_start`
- `write_request_done`
- `read_response_start`
- `read_response_done`
- `connection_reused`
- `connection_idle`

Each event can carry phase, address, method, target, status code, reusability,
timestamp, and error information. The library does not decide whether these
events should be printed as text, JSON, test assertions, or trace spans. That
choice belongs to the caller.

## Current Limits

This classification is intentionally small. It does not yet decide whether an
error is safe to retry in every real-world case, whether a request body was
partially written, or whether a streaming body can be replayed. Those decisions
need more phase detail and request-body metadata.
