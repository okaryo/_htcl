# Connection Management

The current client still opens one TCP connection per request. `Client.Do`
sends `Connection: close` because it does not keep idle connections yet.

This document records the current smaller connection-management steps:

- Decide whether a parsed response says the connection should be closed or
  could be reused later.
- Make the current one-request-per-connection behavior explicit in the client
  implementation.
- Separate one request/response exchange from connection ownership.

## Current Client Lifecycle

`Client.Do` currently follows this sequence:

```text
Dial TCP address
  -> clone request
  -> set Connection: close
  -> serialize request
  -> set write deadline
  -> write request bytes
  -> set read deadline
  -> read one response
  -> close the TCP connection
```

The connection is closed even if the response would be reusable under HTTP/1.1.
This keeps the lifecycle visible before introducing idle connection ownership.

`Client.Do` clones the caller's request before adding `Connection: close`, so
the one-shot client's lifecycle policy does not mutate the request value passed
by the caller.

## Current Exchange Model

`Connection.RoundTrip` sends one request and reads one response on an existing
TCP connection:

```text
mark connection not reusable
serialize request
  -> set write deadline
  -> write request bytes
  -> set read deadline
  -> read one response
  -> mark connection reusable only when request/response allow it
```

Unlike `Client.Do`, `Connection.RoundTrip` does not close the TCP connection.
This makes it possible to study multiple HTTP request/response exchanges on the
same connection before adding an idle connection pool.

`Connection.Reusable` reports whether the connection is still a candidate for
reuse after the last round trip. A connection is not reusable after a write,
read, or parse error. It is also not reusable when either side sends
`Connection: close`.

## Current Reuse Model

`Client.DoReusable` is the first minimal reuse path. It keeps at most one idle
connection per TCP address:

```text
look up idle connection for address
  -> close and discard it when idle timeout has expired
  -> dial when no reusable connection exists
  -> round trip on the connection
  -> keep it when Connection.Reusable is true
  -> close it otherwise
```

This is intentionally smaller than a production connection pool. It is enough
to observe that two requests to the same address can use the same TCP
connection when both responses are fixed-length and neither side requests
`Connection: close`.

Idle connections expire after `Client.IdleTimeout`. When `IdleTimeout` is zero,
the current default is 90 seconds. Expired idle connections are closed before a
new connection is dialed.

## Current Decision Rule

`Response.ShouldCloseConnection` currently applies these HTTP/1.x rules:

- Any `Connection` header containing the `close` token means close the
  connection.
- HTTP/1.1 connections are reusable by default.
- HTTP/1.0 connections close by default.
- HTTP/1.0 connections are reusable only when `Connection: keep-alive` is
  present.
- Unknown versions are treated as close-only.

`Connection` header values are token lists, so values such as
`Connection: upgrade, close` are split on commas and compared
case-insensitively.

## Current Limitation

The current reuse model is intentionally small and single-threaded. It is useful
for observing keep-alive mechanics, but it is not a production connection pool.

Future steps need to account for more conditions before reuse is production-like:

- Multiple idle connections per address.
- Concurrent use protection.
- More response body framing modes such as chunked transfer.

## Current Cancellation Behavior

`DoContext`, `DoReusableContext`, and `RoundTripContext` close the active TCP
connection when the context is canceled. Closing the connection wakes blocked
`Read` or `Write` calls, and the connection is not returned to the idle map.

## Current Resource Cleanup

`Client.CloseIdleConnections` closes all idle TCP connections currently retained
by the minimal reuse path and removes them from the idle map. One-shot requests,
non-reusable responses, errors, cancellations, and expired idle connections also
close their active connection instead of keeping it for reuse.
