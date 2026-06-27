# Cookies

Cookie handling is split across responses and later requests:

- A server sends `Set-Cookie` response headers.
- A client stores selected cookies.
- Later requests send a `Cookie` request header.

## Current Boundary

The current implementation provides simple parsing, formatting, and in-memory
storage:

- `ParseSetCookie` reads the first `name=value` pair from a `Set-Cookie` value
  and recognizes `Domain` and `Path` attributes.
- `CookiesFromSetCookieHeaders` collects cookies from response headers.
- `CookieHeaderValue` formats cookies as a request `Cookie` header value.
- `CookieJar` stores cookies by name and replaces older values with newer
  values that use the same name.
- `CookieJar` can select cookies for a request URL using host-only/domain
  matching and path matching.
- `Max-Age` and `Expires` can prevent expired cookies from being sent.
- Positive `Max-Age` values are converted to an absolute expiration time when
  the cookie is stored. `Max-Age` takes precedence over `Expires`.
- `Max-Age=0` deletes an existing cookie with the same name, domain, and path.

The CLI currently uses this jar while following redirects, so a cookie set by a
redirect response can be sent on the next redirected request.

Security attributes such as `Secure`, `HttpOnly`, and `SameSite` are later
steps.
