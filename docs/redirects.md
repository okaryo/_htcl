# Redirects

Redirect handling starts after an ordinary response has already been parsed.
The transport does not know that a redirect is special; the client looks at the
status code and `Location` header and decides whether to make another request.

## Current Boundary

The current implementation detects redirect responses and resolves the next
URL:

- `IsRedirectStatus` recognizes `301`, `302`, `303`, `307`, and `308`.
- `Response.RedirectLocation` returns the `Location` header only when the
  response status is one of those redirect statuses.
- `ResolveRedirectURL` resolves a relative or absolute `Location` value against
  the request URL using Go's URL parser.
- The CLI can follow redirects for a simple `GET` URL request with `-follow`.
- `-max-redirects` limits how many additional requests can be made while
  following a redirect chain.
- `300 Multiple Choices` and `304 Not Modified` are not treated as automatic
  redirects in this project.

The current follow behavior opens a new one-shot connection for the redirected
request because `Client.Do` still sends `Connection: close`. Method/body
behavior and reusable connections are later steps.
