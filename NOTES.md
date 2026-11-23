# Notes: decisions, omissions, next steps

## Architecture decisions

- **Layered backend, dependencies point inward.** `transport/httpapi → service → domain`; `service` only knows
  repository *interfaces*. That is why the business rules are tested against an in-memory repository (fast, no
  database) while PostgreSQL has its own integration tests.
- **The status workflow lives in one table** (`domain/transitions.go`). The service enforces it, the API returns
  `allowed_transitions` per user, and the UI renders those buttons, so the three can never disagree. An exhaustive
  test checks all 96 role/owner/status combinations against the brief.
- **Authorization is checked in the service, not only in routes.** Middleware gives a first role gate
  (`/api/users` is admin-only), but ticket rules such as "requesters only see their own" need the ticket, so they
  live in the service. A user is re-loaded from the database on every request, so a role change or deletion
  applies immediately instead of when the token expires.
- **Standard library first.** Go 1.22+ method routing replaces a router framework; JWT (HS256) and PBKDF2-SHA256
  hashing use `crypto/*` only. The single third-party Go dependency is the PostgreSQL driver (`lib/pq`).
- **Safe SQL.** Every value is a bind parameter; the only interpolated pieces (sort expression/direction) come
  from a whitelist. `LIKE` wildcards in search input are escaped. Status updates are conditional
  (`WHERE status = <expected>`) so two concurrent transitions cannot both win (optimistic locking, 409).
- **Frontend.** One fetch client owns the token and 401 handling; filters/sort/page live in the URL so refresh and
  back/forward work; one `PageState` component gives every screen the same loading / empty / error behaviour.

## Deliberately left out

- **Attachments, real-time updates (SSE/WebSocket), admin statistics.** Optional in the brief; I preferred a solid core.
- **Rate limiting / lockout on login.** Password checks are slow by design and unknown emails take the same time as
  wrong passwords, but there is no throttle yet.
- **Token revocation / refresh tokens.** Tokens last 24 h and are stored in `localStorage` (simple, but readable by
  injected scripts; an httpOnly cookie + CSRF protection would be stronger).
- **Editing ticket content in the UI, and email notifications.** The API supports content edits.
- **Free-text search beyond titles,** and full-text indexing.

## What I would do with more time

1. Login rate limiting, refresh-token rotation and httpOnly cookies.
2. Audit trail table (who changed status/assignee/priority and when) shown on the ticket.
3. Server-sent events for new comments and status changes; email notification for assigned/resolved tickets.
4. `pg_trgm` index for title search; keyset pagination for very large tables.
5. Attachments with size/type validation and virus scanning; object storage.
6. End-to-end browser tests in CI (Playwright) on top of the existing unit/integration tests; OpenAPI contract tests.
