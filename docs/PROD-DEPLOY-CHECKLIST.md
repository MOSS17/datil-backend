# Prod-deploy checklist (MVP)

> Production domain is `datil.work` (Cloudflare Registrar, ~$8.20/yr). If you pick something else later, find-replace `datil.work` across this file.
>
> See `POST-MVP.md` for everything explicitly out of scope for first launch (WhatsApp, Workspace, marketplace payments, Sentry paid tier, etc.).

## Domain
- [ ] **Register `datil.work`** via Cloudflare Registrar (at-cost ~$8.20/yr). Charges full year upfront.
- [ ] DNSSEC enabled (Cloudflare default).
- [ ] Confirm Cloudflare zone is the authoritative nameserver (auto if registered through Cloudflare).

## Identity / accounts (the new email goes here)
- [ ] New Gmail address (e.g. `datil.ops@gmail.com`) — Workspace upgrade is post-MVP, see `POST-MVP.md`.
- [ ] **Cloudflare Email Routing** configured for `datil.work` — forwards `ops@datil.work`, `noreply@datil.work`, etc. to the Gmail above. Free.
- [ ] **GitHub** account or org owner, repo admin on `MOSS17/datil-backend` + `MOSS17/datil-frontend`.
- [ ] **Google Cloud** account on the new email — for OAuth client (Calendar integration).
- [ ] **Cloudflare** account — for R2, DNS, Pages, Registrar. Single vendor for the stack.
- [ ] **Railway** account on the new email.
- [ ] **Stripe** account on the new email — Mexico entity. Complete identity verification (RFC, business address, bank account for MXN payouts). Allow ~1–3 business days for verification.
- [ ] **Resend** account on the new email — free tier (100 emails/day) covers MVP transactional email (password resets, signup confirmations).
- [ ] Password manager entry for all of the above. 2FA on every account that supports it.

## Infrastructure to provision
- [ ] **R2 bucket** `datil-prod` + custom domain `cdn.datil.work` (CNAME). Public-read.
- [ ] **R2 API token** scoped to that bucket — capture access key + secret once.
- [ ] **Railway project** linked to the GitHub repo, auto-deploy from `main`. Hobby plan ($5/mo) — Postgres bundled.
- [ ] **Postgres** provisioned via Railway add-on. Note: connection string, sslmode=require.
- [ ] **Backups** — Railway Hobby has limited backup capability. Set up a daily `pg_dump` cron pushing to R2 (one-shot worker on Railway, or GitHub Actions). Verify the first snapshot lands.
- [ ] **Production domain DNS** (in Cloudflare zone): `api.datil.work` → Railway, `app.datil.work` → Cloudflare Pages, `cdn.datil.work` → R2.
- [ ] **Google Cloud project** + **OAuth consent screen** + **OAuth 2.0 Client ID** with redirect `https://api.datil.work/api/v1/calendar/google/callback`. Stay in "Testing" mode (<100 users, no review needed).
- [ ] **Stripe Product + Price** — create the subscription product in Stripe Dashboard. One Price object at $150 MXN/mo, recurring monthly. Capture the `price_id`.
- [ ] **Stripe webhook endpoint** registered: `https://api.datil.work/api/v1/webhooks/stripe`. Subscribe to: `customer.subscription.created`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.payment_succeeded`, `invoice.payment_failed`. Capture the signing secret.
- [ ] **Resend domain verification** — add the SPF, DKIM, and DMARC records to the Cloudflare zone. Verify in the Resend dashboard before pointing the backend at it.

## Secrets to set on Railway
Per the runbook in ../PHASES.md — generate fresh, do not reuse dev values:
- [ ] `ENV=production`
- [ ] `DATABASE_URL` (from managed Postgres)
- [ ] `JWT_SECRET` — `openssl rand -base64 48`. Store in password manager.
- [ ] `JWT_ACCESS_EXPIRY=15m`, `JWT_REFRESH_EXPIRY=168h`, `BCRYPT_COST=12`
- [ ] `CORS_ALLOWED_ORIGINS=https://app.datil.work`
- [ ] `STORAGE_PROVIDER=r2`, `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET=datil-prod`, `R2_PUBLIC_BASE_URL=https://cdn.datil.work`
- [ ] `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_REDIRECT_URL=https://api.datil.work/api/v1/calendar/google/callback`
- [ ] `FRONTEND_BASE_URL=https://app.datil.work`, `API_PUBLIC_BASE_URL=https://api.datil.work`
- [ ] `STRIPE_SECRET_KEY` (live mode `sk_live_...`), `STRIPE_WEBHOOK_SECRET` (`whsec_...`), `STRIPE_PRICE_ID` (the $150 MXN/mo subscription)
- [ ] `RESEND_API_KEY`, `RESEND_FROM_ADDRESS=noreply@datil.work`
- [ ] `NOTIFIER_PROVIDER=noop` — explicitly disables the WhatsApp/Twilio path until POST-MVP. (When Twilio is added, swap to `twilio` and set the Twilio vars per `POST-MVP.md`.)

## Frontend prep
- [ ] `VITE_API_BASE_URL=https://api.datil.work/api/v1`
- [ ] `VITE_STRIPE_PUBLISHABLE_KEY` (`pk_live_...`) — needed only if using Stripe Elements; if you're using Stripe-hosted Checkout, skip this.
- [ ] Cloudflare Pages project linked to the frontend repo. Build command `bun run build`, output dir `dist`. Env vars set per environment (Production / Preview). Set `BUN_VERSION` in build settings (or rely on `package.json#packageManager`) so Pages uses Bun, not Node.
- [ ] `app.datil.work` mapped as a custom domain on the Pages project (CNAME in same Cloudflare zone — usually one click).
- [ ] SPA fallback configured — Pages auto-detects Vite's `_redirects` if present; otherwise add `/* /index.html 200` so client-side routes don't 404 on refresh.
- [ ] Build verified locally pointed at staging API before flipping prod DNS.

## Pre-launch verification
- [ ] First Railway deploy succeeds; logs show migrations applied; `/api/v1/auth/signup` round-trip works (auto-verifies for MVP — see `TODO-backend.md`).
- [ ] `PUT /api/v1/business/logo` with a real PNG → response `logo_url` starts with `https://cdn.datil.work`, opens in browser.
- [ ] **Stripe end-to-end**: create a Checkout session via `/billing/checkout-session`, complete payment with a test card in Stripe test mode, confirm webhook fires and updates `businesses.subscription_status = 'active'`. Then repeat with a real card in live mode for one real business.
- [ ] **Stripe Customer Portal**: confirm `/billing/portal-session` returns a working portal URL where a subscriber can update payment method and cancel.
- [ ] **Resend smoke test (auth)**: trigger a password reset email, confirm it arrives at a real inbox (not spam — verify SPF/DKIM/DMARC are aligned).
- [ ] **Resend smoke test (booking)**: complete a real booking on `app.datil.work` against a real business; confirm the customer email arrives with correct business name, logo, services, date/time.
- [ ] Google OAuth full round-trip: connect → revoke → reconnect on a real Google account (not the dev one).
- [ ] `govulncheck` clean on `main` (already gated by CI).
- [ ] Branch protection on `main` requiring `lint`/`vuln`/`test`/`build`.
- [ ] Frontend prod build smoke-tested against prod API — no CORS errors, no mixed content.

## Operational hygiene before public traffic
- [ ] **Sentry free tier** wired up (5k events/mo covers MVP scale). SDK is ~1 hour of work. Paid tier deferred — see `POST-MVP.md`.
- [ ] **UptimeRobot free tier** pinging `/healthz` every 5 min (you'll need to add that endpoint — it's a one-liner; not in the codebase yet).
- [ ] **Stripe webhook observability**: log every webhook event id + type to Sentry (or the DB). Stripe will retry, but silent webhook failures = stale subscription state.
- [ ] `JWT_SECRET` rotation runbook (rotating it kicks every user out — document the steps so future-you isn't improvising at 2am).
- [ ] **Privacy policy + terms-of-service** pages live on the frontend, linked from signup. Mexico has data-protection law (LFPDPPP); consult a lawyer if you're storing customer PII at any scale. Stripe also requires a published refund/cancellation policy.

## What's deliberately *not* in this checklist
Moved to `POST-MVP.md`:
- WhatsApp Business API + Twilio integration (owner notifications, customer confirmations, signup OTP via WhatsApp)
- Google Workspace upgrade
- Marketplace payments (customers paying businesses through Datil)
- Sentry paid tier, BetterStack, status page
- Calendar token encryption at rest
- Staging environment
- Load testing
- Backup-restore drill (do this once before opening signups beyond the hand-onboarded 5)
