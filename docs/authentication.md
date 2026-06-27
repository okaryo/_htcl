# Authentication

Authentication headers are ordinary HTTP request headers. This project starts
with Basic authentication because it is easy to inspect on the wire.

## Basic Authentication

Basic authentication sends:

```text
Authorization: Basic <base64(username:password)>
```

The current implementation provides:

- `BasicAuthorizationValue`, which creates the header value.
- `-basic user:password`, which adds an `Authorization` request header in the
  CLI.

Basic credentials are only base64-encoded, not encrypted. They should only be
used over HTTPS in real clients. HTTPS support is still a later project step.
