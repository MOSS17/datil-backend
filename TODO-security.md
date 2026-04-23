# Security TODOs — Railway deployment

Audit of `/Users/mossandoval/Developer/saas/datil/backend/` against security concerns for a Railway deployment. Ordered roughly by priority (blast radius / exploitability).

## Before first public deploy

### 1. Generate a real `JWT_SECRET`

- **File:** `.env.example:10` — placeholder `change-me-in-production`
- **Risk:** anyone who sees the placeholder can forge tokens for any user. `Signup` is still a stub (`internal/handler/auth.go:29`) but the middleware (`internal/middleware/auth.go:28`) already validates tokens with whatever secret the env provides.
- **Fix:** `openssl rand -base64 32` → set as Railway variable `JWT_SECRET`. Never commit it. Rotate immediately if leaked (invalidates all existing tokens, acceptable pre-launch).

### 2. Force Postgres TLS

- **File:** `.env.example:6` — `sslmode=disable`
- **Risk:** plaintext DB traffic. Railway Postgres requires TLS; the injected `DATABASE_URL` already uses `sslmode=require`. Problem is only if you override it.
- **Fix:** use Railway's injected `DATABASE_URL` verbatim. Don't hardcode `sslmode=disable` anywhere except local dev.

### 3. Lock down CORS

- **File:** `internal/router/router.go:35-42`, `internal/config/config.go:44` default is `http://localhost:3000`
- **Risk:** `AllowCredentials: true` is already set. If `CORS_ALLOWED_ORIGINS` is ever `*` or too broad, any site can ride an authenticated user's session. Today's risk is leaving the default and having prod reject all requests — which you'll notice — but it's easy to misconfigure upward.
- **Fix:** Railway env `CORS_ALLOWED_ORIGINS=https://datil-frontend.vercel.app` (add preview domains only if you need them). Never use `*` with credentials.

### 4. Implement auth with bcrypt + constant-time compare

- **File:** `internal/handler/auth.go:29-64` — all three handlers are stubs
- **Risk:** when you implement, using plain SHA or `==` enables rainbow tables / timing attacks.
- **Fix:** `golang.org/x/crypto/bcrypt` (cost ≥ 12) for hashing. `bcrypt.CompareHashAndPassword` is already constant-time. Never log the password or the hash.

### 5. Refresh-token rotation + revocation

- **File:** `internal/handler/auth.go:56` (`Refresh` stub), `internal/middleware/auth.go:78` (`GenerateTokenPair`)
- **Risk:** current design issues a 168h refresh token. If stolen, attacker gets 7 days of access. No revocation store exists.
- **Fix:** on `/auth/refresh`, issue a new pair AND invalidate the old refresh token (store `jti` in a `refresh_tokens` table, mark used). On logout, delete. This also gives you a kill switch.

### 6. Rate-limit auth + booking + WhatsApp-sending endpoints

- **File:** `internal/router/router.go` — no rate limiter registered. `internal/middleware/` only has `auth.go`.
- **Risk:** credential stuffing against `/auth/login`, spam reservations via `/book/{url}/reserve`, and — most important — if `/auth/signup` or `/book/{url}/reserve` triggers a Twilio WhatsApp send (`internal/notification/twilio.go`), someone can burn your Twilio balance in minutes.
- **Fix:** add `github.com/go-chi/httprate` — e.g. 5 req/min per IP on `/auth/*`, 20/min on `/book/{url}/*`. Tighter limits on anything that sends a WhatsApp message.

## File upload hardening (when implemented)

### 7. `UpdateLogo` and payment-proof uploads

- **Files:** `internal/handler/business.go:37` (`UpdateLogo` stub); payment-proof endpoint not yet on router but referenced by frontend `src/routes/booking/confirmar/components/PaymentProofUploader.tsx`
- **Risks:**
  - Railway containers have ephemeral disk — anything written to local FS is lost on redeploy.
  - No MIME / size validation → zip bombs, oversized uploads, hosted malware.
- **Fix:**
  - Store in object storage, not on disk. Cheapest: **Cloudflare R2** free tier (10GB, no egress) or **Supabase Storage** free tier. Upload via signed URL or multipart → stream to R2 → save only the URL in Postgres.
  - Validate: `Content-Type` in allowlist (`image/jpeg`, `image/png`, `image/webp`, `application/pdf` for proofs), `Content-Length` ≤ 5MB, sniff magic bytes server-side (don't trust the header).
  - Rename on upload (UUID filename) to prevent path traversal and collisions.

## Operational hygiene

### 8. Non-root container user

- **File:** `Dockerfile` — no `USER` directive, runs as root.
- **Risk:** defense in depth. Not critical on Railway's managed runtime but cheap to fix.
- **Fix:** add `RUN adduser -D -u 10001 app && chown -R app /app` and `USER app` in the final stage.

### 9. Log hygiene

- **File:** `internal/router/router.go:32` uses `chimw.Logger` (chi's default)
- **Risk:** chi's Logger logs method + URL + status + duration — safe by default. But once auth handlers are implemented, don't log request bodies on `/auth/*` or `Authorization` headers anywhere. Railway logs are visible to any teammate with project access.
- **Fix:** keep bodies out of the logger. If you add request-body logging for debug, gate it on `cfg.Env == "development"`.

### 10. Twilio webhook signature verification

- **File:** no webhook route exists yet; `internal/notification/twilio.go` is outbound only.
- **Risk:** when you add delivery-status webhooks, unsigned requests can be forged.
- **Fix:** verify `X-Twilio-Signature` per [Twilio docs](https://www.twilio.com/docs/usage/webhooks/webhooks-security) before processing.

### 11. Keep Postgres internal-only on Railway

- **Risk:** if public networking is toggled on the Postgres service, the DB is exposed to the internet with only password auth.
- **Fix:** use Railway's private `DATABASE_URL` (default). For one-off migrations from your laptop, use `railway connect` (tunnel) instead of exposing publicly.

### 12. Migration strategy

- **File:** `migrations/` (raw SQL, presumably run via `golang-migrate`)
- **Risk:** manual `railway run make migrate` from a laptop is fine for now but easy to forget; a migration-less deploy with a schema change = runtime errors.
- **Fix:** either add a Railway pre-deploy command that runs migrations, or wire a one-shot startup hook in `cmd/api/main.go` that runs `migrate.Up()` before the HTTP server starts. Idempotent either way.

## Deferrable (revisit before real users)

- `govulncheck` in CI — catches known vulns in Go deps
- Structured logging with request IDs propagated to errors (you already have `chimw.RequestID`, just need to include it in error responses)
- CSRF — not needed while auth is Bearer-token in `Authorization` header (not cookies). Becomes relevant if you move to cookie sessions.
- Content Security Policy headers on API responses — low value for a JSON API, skip
- Secret rotation playbook — document how to rotate `JWT_SECRET`, `TWILIO_AUTH_TOKEN`, DB password
- Backup testing — Railway Postgres has automatic backups; verify restore works once before launch

## Frontend cleanup (related)

- `src/auth/AuthProvider.tsx:10` — `DEV_BYPASS_AUTH = true` unconditionally. Revert to `import.meta.env.DEV` (or gate on `VITE_AUTH_BYPASS`) before pointing the frontend at a real backend.
- `src/api/mocks/router.ts:14` — `MOCKS_ENABLED` currently defaults on in prod. Set Vercel env `VITE_API_MOCKS=false` when real backend is live.
