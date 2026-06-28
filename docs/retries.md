# Retries And Idempotency

This step adds a small retry mechanism for observing when automatic retries are
reasonable and when they are risky.

## Idempotent Methods

An idempotent method is expected to have the same intended effect when the same
request is applied once or multiple times.

This project treats these methods as idempotent:

- `GET`
- `HEAD`
- `PUT`
- `DELETE`
- `OPTIONS`
- `TRACE`

It treats these as non-idempotent by default:

- `POST`
- `PATCH`
- unknown extension methods

This does not mean every `GET` or `DELETE` is harmless in the real world. It
means the HTTP method semantics make retrying less dangerous than retrying a
typical `POST`.

## Current CLI Behavior

The CLI accepts `-retries` for URL-based requests:

```sh
go run ./cmd/htcl -retries 1 http://127.0.0.1:8080/unstable
```

`-retries 1` means the command may make one retry after the first failed
attempt. The total number of attempts can therefore be two.

Retries are only attempted when the method is idempotent:

```sh
go run ./cmd/htcl -retries 1 -method GET http://127.0.0.1:8080/unstable
```

The same flag does not retry a `POST`:

```sh
go run ./cmd/htcl -retries 1 -method POST -body hello http://127.0.0.1:8080/submit
```

## Why Retries Are Hard

The current implementation retries after request/response exchange errors, but
it does not yet classify whether the failure happened before the request bytes
were written, while writing them, or after the server already received them.

That distinction matters:

- If dialing fails, the origin probably never saw the request.
- If writing fails halfway through, the server may have received a partial
  request.
- If reading the response fails, the server may already have completed the
  request.

Because this project has not implemented detailed error classification yet,
retry behavior is intentionally opt-in and limited to idempotent methods.

## Future Work

More realistic retry behavior would classify errors by phase, respect
`Retry-After`, apply backoff, and avoid retrying requests with non-replayable
streaming bodies.
