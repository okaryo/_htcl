# Cookies

Cookie handling is split across responses and later requests:

- A server sends `Set-Cookie` response headers.
- A client stores selected cookies.
- Later requests send a `Cookie` request header.

## Current Boundary

The current implementation provides simple parsing, formatting, and in-memory
storage:

- `ParseSetCookie` reads the first `name=value` pair from a `Set-Cookie` value
  and ignores attributes such as `Path`, `Max-Age`, and `HttpOnly`.
- `CookiesFromSetCookieHeaders` collects cookies from response headers.
- `CookieHeaderValue` formats cookies as a request `Cookie` header value.
- `CookieJar` stores cookies by name and replaces older values with newer
  values that use the same name.

The CLI currently uses this jar while following redirects, so a cookie set by a
redirect response can be sent on the next redirected request.

Domain matching, path matching, expiration, deletion, and security attributes
are later steps.
