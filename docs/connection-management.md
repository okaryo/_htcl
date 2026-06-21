# Connection Management

The current client still opens one TCP connection per request and sends
`Connection: close`.

This document records the first smaller connection-management step: deciding
whether a parsed response says the connection should be closed or could be
reused later.

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
