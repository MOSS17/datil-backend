# Backend TODOs

Endpoints the frontend (`../frontend/`) depends on. Most of the routes below already exist in `internal/router/router.go` but the handlers are stubs (`ErrorJSON(w, http.StatusNotImplemented, ...)`); a few routes don't exist yet at all. This doc is the source of truth for the frontend ↔ backend contract — match the shapes here exactly or the frontend hooks will break.

All paths are relative to the server root. Frontend calls them through `VITE_API_BASE_URL` (default `http://localhost:8080/api/v1`). JWT auth via `Authorization: Bearer <token>` unless marked public. Error responses must use the shape `{ message: string, errors?: Record<string, string> }` — the frontend throws that as `ApiError`.

See also `TODO-security.md` for cross-cutting hardening (rate limiting, JWT secret, CORS, file uploads).

---

## Auth

### `POST /auth/signup` — handler stub

- **Status:** route exists (`router.go:46`), handler is a stub (`handler/auth.go:29`).
- **Name mismatch to resolve:** frontend hook `useRegister` currently posts to `/auth/register` (see `../frontend/src/api/types/auth.ts::RegisterRequest`). Pick one — `/auth/signup` in the router is fine; update the frontend endpoint constant to match, or rename the route. **Action:** align naming before shipping either side.
- **Request body:** `{ name, email, password, business_name }`. Create the user's `business` record with `business_name` in the same transaction as the user.
- **Response:** no token issued here. User is unverified until `/auth/verify-email` succeeds. Return `204` or the created user with an `is_verified: false` flag — pick one and document.
- **Implementation notes already stubbed in `handler/auth.go:29-40`:** bcrypt the password (cost ≥ 12), wrap business + user creation in a transaction via `repository.WithTransaction`.

### `POST /auth/verify-email` — new route needed

- **Used by:** `useVerifyEmail` (`../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ email: string, code: string }` (6-digit numeric).
- **Success (200):** `{ token: string, user: User }` — same `AuthResponse` shape as login. The frontend logs the user in and redirects to `/dashboard`.
- **Failure (400):** `{ message }` with a user-facing Spanish string (e.g. "El código no es válido" or "El código ha expirado"). Frontend surfaces `message` under the OTP input.
- **Auth:** public.
- **Notes:** code storage should have a TTL (e.g. 10 min). Invalidate on use. Consider bcrypt-hashing the stored code if stored in DB (it's low-entropy).

### `POST /auth/resend-code` — new route needed

- **Used by:** `useResendCode` (`../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ email: string }`.
- **Success (204):** no body. Frontend shows "Te enviamos un nuevo código.".
- **Auth:** public.
- **Rate limit:** 1 request per 60 s per email (see `TODO-security.md` item 6). This sends a Twilio WhatsApp message — unrate-limited = burnt Twilio balance.

### `POST /auth/forgot-password` — new route needed

- **Used by:** `useForgotPassword` (`../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ email: string }`.
- **Success (204):** **always return 204 regardless of whether the email exists** — prevents account enumeration. If the email matches a registered user, send a reset link pointing at `https://<frontend>/login/nueva-contrasena?token=<opaque-or-jwt>`.
- **Auth:** public.
- **Rate limit:** 3 requests per 15 min per email.
- **Token:** single-use, time-limited (1 h). Store in a `password_reset_tokens` table with `used_at` nullable.

### `POST /auth/reset-password` — new route needed

- **Used by:** `useResetPassword` (`../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ token: string, password: string }`.
- **Success (204):** no body. Frontend navigates to `/login?reset=success`.
- **Failure (400):** `{ message }` in Spanish for invalid / expired token. Surfaces under the password field.
- **Auth:** public. Mark the token `used_at = now()` on success.

### `POST /auth/login` — handler stub

- **Status:** route exists (`router.go:47`), handler is a stub (`handler/auth.go:44`). Implementation steps are already listed in the stub comment.

### `POST /auth/refresh` — handler stub

- **Status:** route exists (`router.go:48`), handler is a stub (`handler/auth.go:56`).
- **Security note:** `TODO-security.md` item 5 recommends rotating refresh tokens on each use (store `jti` in a `refresh_tokens` table, mark used, issue a new pair). Do this from the start rather than retrofitting.

---

## Business

### `POST /businesses/:id/logo` — handler stub (route shape differs from frontend)

- **Status:** route exists as `PUT /business/logo` (`router.go:66`, singular, no ID), handler is a stub (`handler/business.go:37`). Frontend expects `POST /businesses/:id/logo` — pick one shape.
- **Recommended:** keep the router's `PUT /business/logo` (the `:id` is redundant because the authenticated user's business is already derivable from the JWT's `business_id` claim). **Action:** update `../frontend/src/api/hooks/useBusiness.ts::useUploadBusinessLogo` to match.
- **Body:** `multipart/form-data` with a single `logo` field. Accept PNG / JPEG / WebP up to 2 MB.
- **Success (200):** updated `Business` (including new `logo_url`). Frontend invalidates `businessKeys.all`.
- **Auth:** authenticated; must own the business (enforced by the JWT `business_id` claim).
- **Storage:** per `TODO-security.md` item 7, store in Cloudflare R2 / Supabase Storage, not on the container filesystem. Persist only the public URL in `businesses.logo_url`.

---

## Services

### Service extras linkage — handlers stubbed, one route missing

- **Routes already registered in `router.go:83-84`:**
  - `POST /services/{id}/extras` — body `{ extra_id: string }` to attach an extra. Handler is a stub (`handler/service.go::LinkExtra`).
  - `DELETE /services/{id}/extras/{extraId}` — detach. Handler is a stub (`handler/service.go::UnlinkExtra`).
- **Route missing:** `GET /services/{id}/extras` — returns the service's currently-linked extras. Add it alongside the existing two.
- **Used by:** `ServiceFormPage` (`../frontend/src/routes/dashboard/servicios/ServiceFormPage.tsx:92`). The form already collects `values.extrasGroupIds` but drops it on submit because there's nowhere to send it yet.
- **Frontend follow-up once shipped:** add `useServiceExtras`, `useAttachExtra`, `useDetachExtra` hooks (sketched in `../frontend/.claude/skills/datil-figma-to-code/references/api-patterns.md`) and diff-apply on submit in `ServiceFormPage`.
- **Auth:** authenticated; must own the service's business.

---

## Public booking flow — all handlers stubbed

Frontend built against these in the `feat/customer-facing-store` branch (`../frontend/src/routes/booking/*`). Routes exist in `router.go:51-56`, all handlers return `501 Not Implemented` (`handler/booking.go`).

Needed shapes (frontend types live in `../frontend/src/api/types/`):

- `GET /book/{url}` — returns public `Business` (name, logo_url, slug, bank details for manual transfer, list of categories).
- `GET /book/{url}/services` — returns services grouped by category. Include each service's linked extras.
- `GET /book/{url}/availability?date=YYYY-MM-DD&service_ids=...` — returns available time slots for the given date, accounting for workdays, personal time, and existing appointments.
- `POST /book/{url}/reserve` — creates a reservation. Body: customer info (name, phone, email), selected services + extras, date, start time, optional payment-proof URL (uploaded separately or inline multipart). Returns the created `Appointment`. Should trigger a Twilio WhatsApp notification to the business owner (see `internal/notification/twilio.go`).

**Payment-proof upload endpoint not yet on the router.** Frontend uploads via `PaymentProofUploader` (`../frontend/src/routes/booking/confirmar/components/PaymentProofUploader.tsx`). Decide: inline multipart in `/reserve`, or a separate `POST /book/{url}/upload-proof` that returns a URL the reserve call references. Same R2/Storage guidance as the logo endpoint.

---

## Frontend-side cleanup (coordinated with backend go-live)

These aren't backend work, but they gate the real API going live. Tracked here so backend readers know what needs to happen across the repo boundary before flipping off mocks.

- **`../frontend/src/auth/AuthProvider.tsx:10`** — `DEV_BYPASS_AUTH = true` unconditionally. Revert to `import.meta.env.DEV` (or gate on `VITE_AUTH_BYPASS`) before pointing the frontend at a real backend.
- **`../frontend/src/api/mocks/`** + `resolveMock` call in `../frontend/src/api/client.ts`. Delete the `mocks/` directory and the resolver call once every endpoint in this doc is shipped and the frontend is verified against the real API. Until then, set `VITE_API_MOCKS=false` in Vercel to disable at runtime.

---

## Keeping this doc honest

- Every frontend item above is pinned to a `TODO(backend)` or `TODO(mocks)` comment in the frontend source.
- From the frontend repo root: `grep -rn "TODO(backend)\|TODO(mocks)" src/` — every match should have a section here.
- From this (backend) repo root: `grep -rn "TODO: implement\|not implemented" internal/` — every stub should be covered here or in `TODO-security.md`.
- When an endpoint ships: delete the frontend comment, delete the corresponding section here, and update `../frontend/TODO-backend.md` if it still exists (it's being deprecated in favor of this one).

### Known frontend TODOs *not* tracked here

- `../frontend/src/routes/dashboard/configuracion/ConfiguracionPage.tsx:32` — `TODO(autosave)`. No backend work required; the existing `PATCH /businesses/:id` endpoint is sufficient. Product/UX decision, tracked in design backlog.
