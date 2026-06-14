# Raw TCP HTTP GET

The first implementation step uses `net.Dial` directly and writes an HTTP/1.1
request as bytes.

The current sequence is:

```text
net.Dialer.Dial("tcp", address)
  -> SetWriteDeadline
  -> write "GET ... HTTP/1.1\r\n"
  -> write headers and the empty line
  -> SetReadDeadline
  -> read raw response bytes until EOF
  -> print those bytes without parsing
```

## Current Request

The client currently writes a fixed GET request:

```text
GET <target> HTTP/1.1
Host: <host>
User-Agent: htcl/0.1
Connection: close
```

The blank line after the headers terminates the request header section.

`Connection: close` is intentional in this step. It gives the client a simple
completion signal: after the server sends the response and closes the
connection, `Read` returns `io.EOF`.

## Blocking Points

The current command logs before the major blocking operations:

- `Dial` may block while the TCP connection is established.
- `Write` may block if the socket cannot accept more bytes.
- `Read` blocks while waiting for response bytes.

The command applies the same timeout value to dialing, writing, and reading so
these blocking points are easier to observe.

## Current Limitations

- Only plain TCP HTTP is supported.
- HTTPS is not supported yet.
- Only GET requests are written.
- The response is not parsed yet.
- `Content-Length` and chunked response framing are not interpreted yet.
- The client relies on `Connection: close` and EOF to know that the response is
  complete.
