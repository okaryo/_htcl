# Cookies

Cookie handling is split across responses and later requests:

- A server sends `Set-Cookie` response headers.
- A client stores selected cookies.
- Later requests send a `Cookie` request header.

## Current Boundary

The current implementation only provides the parsing and formatting pieces:

- `ParseSetCookie` reads the first `name=value` pair from a `Set-Cookie` value
  and ignores attributes such as `Path`, `Max-Age`, and `HttpOnly`.
- `CookiesFromSetCookieHeaders` collects cookies from response headers.
- `CookieHeaderValue` formats cookies as a request `Cookie` header value.

There is no cookie jar yet. Domain matching, path matching, expiration,
replacement, deletion, and security attributes are later steps.
