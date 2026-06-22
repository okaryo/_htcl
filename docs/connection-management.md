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
serialize request
  -> set write deadline
  -> write request bytes
  -> set read deadline
  -> read one response
```

Unlike `Client.Do`, `Connection.RoundTrip` does not close the TCP connection.
This makes it possible to study multiple HTTP request/response exchanges on the
same connection before adding an idle connection pool.

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

This step only decides whether a connection is theoretically reusable. The
client does not actually reuse connections yet.

Future steps need to account for more conditions before reuse is safe:

- The response body must be completely read.
- The parser must know where the response body ends.
- The connection must not have hit a network, timeout, or protocol error.
- The client must not have requested `Connection: close`.
- Idle connections need ownership and cleanup rules.
