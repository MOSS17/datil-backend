# Backend TODOs (MVP)

Endpoints the frontend (`../../frontend/`) depends on that are not yet implemented, and Stripe billing work needed before launch. This doc is the source of truth for the frontend ↔ backend contract on these — match the shapes here exactly or the frontend hooks will break.

All paths are relative to the server root. Frontend calls them through `VITE_API_BASE_URL` (default `http://localhost:8080/api/v1`). JWT auth via `Authorization: Bearer <token>` unless marked public. Error responses must use the shape `{ message: string, errors?: Record<string, string> }` — the frontend throws that as `ApiError`.

See also `TODO-security.md` for cross-cutting hardening (rate limiting, JWT secret, CORS, file uploads). See `POST-MVP.md` for items deliberately out of scope (WhatsApp signup OTP, marketplace payments, etc.).

---

## Auth — MVP behavior changes & password reset

### `POST /auth/signup` — auto-verify behavior change

- **Status:** route + handler exist and work. Currently designed around "user is unverified until `/auth/verify-email` succeeds" with a WhatsApp-delivered code.
- **MVP change:** since WhatsApp is deferred (see `POST-MVP.md`), signup must issue a JWT immediately and return the standard `AuthResponse` (`{ token, user }`). User is logged in directly.
- **Action:** confirm the implementation issues a token on signup; if it currently returns `204` waiting for verification, update it to return `AuthResponse`.
- **Rationale:** at MVP scale (5 hand-vetted businesses), email/phone verification is over-engineered. Re-enable verification when WhatsApp ships per `POST-MVP.md`.

### `POST /auth/forgot-password` — new route needed (email-based)

- **Used by:** `useForgotPassword` (`../../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ email: string }`.
- **Success (204):** **always return 204 regardless of whether the email exists** — prevents account enumeration. If the email matches a registered user, send a reset link via Resend pointing at `https://app.datil.work/login/nueva-contrasena?token=<opaque-or-jwt>`.
- **Auth:** public.
- **Email delivery:** use Resend (`RESEND_API_KEY`, `RESEND_FROM_ADDRESS=noreply@datil.work`). Free tier (100/day) covers MVP.
- **Rate limit:** 3 requests per 15 min per email.
- **Token:** single-use, time-limited (1 h). Store in a `password_reset_tokens` table with `used_at` nullable.

### `POST /auth/reset-password` — new route needed

- **Used by:** `useResetPassword` (`../../frontend/src/api/hooks/useAuth.ts`).
- **Body:** `{ token: string, password: string }`.
- **Success (204):** no body. Frontend navigates to `/login?reset=success`.
- **Failure (400):** `{ message }` in Spanish for invalid / expired token. Surfaces under the password field.
- **Auth:** public. Mark the token `used_at = now()` on success.

---

## Stripe subscription billing — new module

The product launches behind a $150 MXN/mo subscription. All endpoints below are new; nothing exists yet in `internal/billing/` or `internal/handler/billing.go`.

### Schema additions to `businesses` table

Add columns (single migration):
- `stripe_customer_id text` (nullable, unique when set)
- `stripe_subscription_id text` (nullable)
- `subscription_status text` (nullable; mirrors Stripe values: `active`, `past_due`, `canceled`, `trialing`, `incomplete`, `incomplete_expired`, `unpaid`)
- `subscription_current_period_end timestamptz` (nullable)

Optional but recommended: a `subscription_events` audit table (event_id, business_id, event_type, payload jsonb, received_at) — Stripe retries webhooks; idempotent ingestion needs `event_id` uniqueness.

### `POST /billing/checkout-session` — new route

- **Used by:** the billing/onboarding page on the frontend (TBD — coordinate with frontend; this hook does not exist yet).
- **Body:** none (subscription tier is fixed at MVP — single price).
- **Behavior:** creates a Stripe Customer if `businesses.stripe_customer_id` is null, then creates a Stripe Checkout Session in subscription mode for `STRIPE_PRICE_ID`, with `success_url=https://app.datil.work/billing/success?session_id={CHECKOUT_SESSION_ID}` and `cancel_url=https://app.datil.work/billing/cancel`.
- **Success (200):** `{ checkout_url: string }`. Frontend redirects.
- **Auth:** authenticated; uses the JWT's `business_id`.

### `POST /billing/portal-session` — new route

- **Used by:** "Manage subscription" link on the dashboard.
- **Body:** none.
- **Behavior:** creates a Stripe Billing Portal session for the business's `stripe_customer_id`. Lets the customer update payment method, see invoices, cancel.
- **Success (200):** `{ portal_url: string }`. Frontend redirects.
- **Auth:** authenticated; requires `stripe_customer_id` to be set (otherwise 400 — they haven't checked out yet).

### `POST /webhooks/stripe` — new route

- **Auth:** public, but verify the Stripe signature using `STRIPE_WEBHOOK_SECRET`. Reject with 400 on invalid signature.
- **Handle these event types** (others can no-op):
  - `customer.subscription.created` / `customer.subscription.updated` → upsert `stripe_subscription_id`, `subscription_status`, `subscription_current_period_end` on the matching business.
  - `customer.subscription.deleted` → set `subscription_status = 'canceled'`.
  - `invoice.payment_succeeded` → confirm `subscription_status = 'active'` (Stripe sometimes sends this before the subscription event lands).
  - `invoice.payment_failed` → already reflected via `customer.subscription.updated` going `past_due`; log it for visibility.
- **Idempotency:** check `subscription_events.event_id`; if already processed, return 200 without reapplying.
- **Response:** always 200 on a valid signature (even if the event is one we don't handle), so Stripe stops retrying.

### Subscription gate middleware

- **Behavior:** authenticated routes (everything under `/api/v1/` that requires JWT, except `/billing/*`) check `businesses.subscription_status`. If not in `{active, trialing}`, return `402 Payment Required` with `{ message: "Tu suscripción no está activa", subscription_status }`.
- **Grace period:** allow `past_due` for 3 days (compare `subscription_current_period_end` + 3d > now()) before blocking. Stripe retries failed payments for several days; don't lock people out instantly.
- **Frontend coordination:** the frontend should catch `402` globally and redirect to a "renew subscription" screen with a button hitting `/billing/portal-session`.

### Public booking flow stays open

- The `/book/{url}/*` public endpoints are *not* gated — customers booking with a business shouldn't see "subscription expired." Instead, gate the *business owner's* dashboard and let bookings continue. (Decide separately whether to send a notification telling the owner their account is suspended; not MVP.)

---

## Services

### `GET /services/{id}/extras` — new route needed

- **Status:** link/unlink (`POST /services/{id}/extras`, `DELETE /services/{id}/extras/{extraId}`) are implemented. The read endpoint to fetch a service's currently-linked extras is not.
- **Used by:** `ServiceFormPage` (`../../frontend/src/routes/dashboard/servicios/ServiceFormPage.tsx`) — needed to hydrate the form when editing an existing service.
- **Frontend follow-up once shipped:** add `useServiceExtras` hook and diff-apply on submit in `ServiceFormPage` (sketched in `../../frontend/.claude/skills/datil-figma-to-code/references/api-patterns.md`).
- **Auth:** authenticated; must own the service's business.

---

## Public booking flow

Booking routes are registered (`router.go:72`) and the core handlers (`/book/{url}`, `/services`, `/availability`, `/reserve`) are implemented. Two open items:

### Payment-proof shape decision

- **Payment-proof upload endpoint not yet on the router.** Frontend uploads via `PaymentProofUploader` (`../../frontend/src/routes/booking/confirmar/components/PaymentProofUploader.tsx`). Decide: inline multipart in `/reserve`, or a separate `POST /book/{url}/upload-proof` that returns a URL the reserve call references. Same R2/Storage guidance as the logo endpoint (see `TODO-security.md` item 7).

### Customer booking confirmation email — new behavior on `/reserve`

- **Status:** `/book/{url}/reserve` succeeds but does not currently send any notification. WhatsApp confirmations are deferred (`POST-MVP.md`); email is the MVP delivery channel.
- **Behavior:** after a successful reserve, send a Spanish-language confirmation email via Resend to the customer email collected in the request body. Use a branded sender like `"{Business Name} <noreply@datil.work>"`. Include: business name + logo (link to `cdn.datil.work/...`), services booked, date/time, business address (if set in `businesses`), business phone, and a "para cancelar contáctanos al..." line.
- **Failure handling:** if Resend errors, log it but **do not fail the booking**. The reserve already succeeded; notification is best-effort. Surface the failure to Sentry so we know if the email path quietly breaks.
- **Template:** keep simple — inline HTML + plain-text alternative, generated by Go's `html/template`. No Resend dynamic templates for MVP.
- **Volume:** at 5 businesses × 50 appts/mo = 250 emails/mo, easily inside Resend free tier (3000/mo, 100/day).
- **Owner notification design:** owners are *intentionally* not emailed or WhatsApped on new bookings — the goal is to keep their inboxes clean. Instead the dashboard sidebar shows an unseen-bookings badge driven by `GET /appointments/unseen-count` (sibling to `POST /appointments/{id}/seen`).

### `POST /appointments/seen` — optional bulk mark-as-seen

- **When to add:** only if the dashboard introduces a "mark all as seen" affordance, or marks the visible list seen in one shot when the home tab is opened.
- **Body:** `{ ids: string[] }`.
- **Behavior:** sets `seen_at = now()` for each id where `seen_at IS NULL` and the appointment belongs to the authenticated business. Ignore unknown / already-seen ids silently.
- **Success (204):** no body.
- **Skip if:** the frontend marks each card seen as it scrolls into view (per-id endpoint is fine).

---

## Frontend-side cleanup (coordinated with backend go-live)

These aren't backend work, but they gate the real API going live. Tracked here so backend readers know what needs to happen across the repo boundary before flipping off mocks.

- **`../../frontend/src/auth/AuthProvider.tsx`** — `DEV_BYPASS_AUTH = true` unconditionally. Revert to `import.meta.env.DEV` (or gate on `VITE_AUTH_BYPASS`) before pointing the frontend at a real backend.
- **`../../frontend/src/api/mocks/`** + `resolveMock` call in `../../frontend/src/api/client.ts`. Delete the `mocks/` directory and the resolver call once every endpoint in this doc is shipped and the frontend is verified against the real API. Until then, set `VITE_API_MOCKS=false` in Cloudflare Pages env to disable at runtime.
- **Subscription gate UI** — frontend needs a "renew subscription" screen that's reachable when the API returns `402`, with a button calling `/billing/portal-session` and redirecting to the returned URL. Coordinate when the billing endpoints land.
- **Strip the email-verify / OTP screens** that depend on `useVerifyEmail` / `useResendCode` — those endpoints are post-MVP. Either hide the screens or skip them in the signup flow until WhatsApp ships.

---

## Keeping this doc honest

- Every frontend item above is pinned to a `TODO(backend)` or `TODO(mocks)` comment in the frontend source.
- From the frontend repo root: `grep -rn "TODO(backend)\|TODO(mocks)" src/` — every match should have a section here or in `POST-MVP.md`.
- From this (backend) repo root: `grep -rn "TODO: implement\|not implemented" internal/` — every stub should be covered here or in `TODO-security.md`.
- When an endpoint ships: delete the frontend comment, delete the corresponding section here.

### Known frontend TODOs *not* tracked here

- `../../frontend/src/routes/dashboard/configuracion/ConfiguracionPage.tsx` — `TODO(autosave)`. No backend work required; the existing `PATCH /businesses/:id` endpoint is sufficient. Product/UX decision, tracked in design backlog.
