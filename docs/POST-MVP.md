# Post-MVP work

Catalog of everything deliberately deferred from MVP launch. For each item: what it is, why it's deferred, what would trigger reprioritization, and what work is required to ship it.

MVP scope is in `PROD-DEPLOY-CHECKLIST.md` and `TODO-backend.md`. If something is *not* in those files but *is* in this one, it is not blocking launch.

---

## Communications

### WhatsApp Business API integration (Twilio)

- **What:** Outbound WhatsApp notifications — owner gets a message when a customer books, customer reminders before appointment, optional WhatsApp customer confirmations (upgrade over the email confirmation that ships at MVP), signup OTP delivery.
- **Why deferred:**
  - Meta business verification takes weeks and may require multiple back-and-forths. Blocks launch if held as a hard dependency.
  - Adds ~$0.013/message ongoing per appointment notification (~$3/mo at 5 businesses, ~$65/mo at 100). Real cost, not infinite, but unjustified before there's revenue.
  - **Customer booking confirmations now ship via email at MVP** (see `TODO-backend.md`), so the customer-side notification gap is filled. WhatsApp customer confirmations become a delivery-channel upgrade (better engagement in MX where WhatsApp is dominant) rather than a missing feature.
  - **Owner booking notifications are intentionally *not* in MVP** (or post-MVP) — the design goal is to keep owner inboxes clean. New bookings surface via an unseen-count badge on the dashboard sidebar, backed by `seen_at` + `GET /appointments/unseen-count` (see `TODO-backend.md`). WhatsApp owner notifications would only be added later if owners ask for push-style alerts away from the dashboard.
- **What triggers re-prioritization:**
  - Owners ask for phone notifications (most common ask in MX market — set expectations now).
  - No-show rate becomes a measurable problem (reminders are the proven mitigation).
  - 20+ paying businesses, where WhatsApp UX becomes a competitive differentiator vs Calendly clones.
- **Required work:**
  - Twilio account upgraded out of trial; WhatsApp sender approved (sandbox for staging, prod = Meta Business verification).
  - Implement notification triggers: booking created → owner; reminder cron 24h before appointment → customer (if customer has phone).
  - Restore `POST /auth/verify-email` and `POST /auth/resend-code` endpoints (originally specified in `TODO-backend.md` history; designed around WhatsApp delivery).
  - Twilio template approval for utility messages (Mexico-region templates, Spanish, business name interpolation).
  - Per-business toggle to enable/disable notifications in the dashboard.
  - Rate limiting on `/auth/resend-code` (1 req per 60 s per email) to prevent burnt Twilio balance.
  - Update `NOTIFIER_PROVIDER` env var from `noop` to `twilio`; set `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_WHATSAPP_FROM`.
  - Re-enable email verification in signup flow (or switch to email OTP via Resend if Meta approval drags).

### Google Workspace upgrade

- **What:** Move from free Cloudflare Email Routing to Google Workspace Business Starter (~$7.20 USD/mo per seat, ~$122 MXN).
- **Why deferred:** Email Routing covers receiving for `ops@datil.work`, `noreply@datil.work`, etc. and forwards to a personal Gmail. The cost is real until product is generating revenue.
- **What triggers re-prioritization:**
  - WhatsApp Business API approval (Meta reviewers will email — Workspace removes friction by giving you a real send-from-domain inbox with native SPF/DKIM).
  - Hiring a second person who needs `their.name@datil.work`.
  - Wanting Calendar/Drive/Meet at the business domain for client meetings.
- **Required work:** Buy Workspace seat, verify domain ownership, switch MX records from Email Routing to Workspace, migrate any saved mail.

---

## Payments

### Marketplace flow — customers paying businesses through Datil

- **What:** End customers pay for appointments at booking time; Datil takes a fee and routes the rest to the business's bank account. Distinct from MVP's SaaS subscription billing (where *businesses* pay *Datil* $150 MXN/mo).
- **Why deferred:** Tier-1 SaaS billing is the immediate revenue path. Marketplace is a separate product with its own onboarding, compliance, and dispute handling.
- **What triggers re-prioritization:**
  - Repeated customer demand for prepayment / no-show deposits.
  - Competitive pressure from booking platforms with native payment.
  - A specific business segment (e.g. spas, beauty salons) where prepayment is normal expectation.
- **Required work:**
  - Decide: Stripe Connect (Express accounts) vs Mercado Pago. Reopens the Stripe-vs-MP question for the consumer-facing side, where MP's *meses sin intereses* and brand recognition matter more than they did for SaaS billing. See `memory/project_payments.md` for full context.
  - Schema additions: businesses gain a `stripe_account_id` (or MP equivalent); each appointment links to a `payment_intent_id`.
  - Webhook handlers for: payment intent succeeded/failed, transfers to connected accounts, refunds, disputes.
  - Refund/cancellation policy enforcement at the API level (e.g. "refundable until 24h before appointment").
  - Onboarding flow for businesses to connect their bank/Stripe Connect account.
  - Tax handling: who issues the IVA receipt — Datil or the business? Talk to an accountant before shipping.

---

## Operational hardening

### Sentry paid tier

- **What:** Move from free 5k events/mo to Team plan (~$26 USD/mo).
- **Why deferred:** Free tier covers MVP scale comfortably.
- **When:** Cross 5k events sustained (typically around 50–100 active businesses with normal noise), or hit the volume needing per-issue alerting and longer retention.

### Calendar token encryption at rest

- **What:** Encrypt Google Calendar OAuth refresh tokens in the DB at the application layer (not just Postgres-at-rest).
- **Why deferred:** Low risk at 5 hand-vetted businesses with disk-encrypted Postgres on Railway. Meaningful at scale, especially as a security/compliance signal for larger customers.
- **Required work:** Pick a KMS (Railway secrets are fine for the encryption key; rotate-friendly). Encrypt-at-write, decrypt-at-read in the calendar token repository. Migration script to encrypt existing rows.

### Staging environment

- **What:** Separate Railway project + R2 bucket + Postgres for staging. Swappable into the frontend via `VITE_API_BASE_URL`.
- **Why deferred:** Cost (~$5–10/mo extra Railway) and limited value at zero users.
- **When:** Once the first customer-impacting bug ships to prod that staging would have caught — that's the signal you've graduated past "test in prod is fine."
- **Bonus:** if you've also moved to Neon Postgres by then, branching makes staging effectively free per-PR.

### Backup-restore drill

- **What:** Actually restore a Postgres snapshot to a fresh DB and verify the app works against it.
- **Why "deferred":** One-time exercise but worth doing before claiming production-readiness.
- **When:** Before publicly opening signups beyond the hand-onboarded 5. Untested backups are not backups.

### Status page

- **What:** Public status page (BetterStack, statuspage.io, or self-hosted).
- **When:** Skip until the first real outage proves you need one. Pre-launch you don't have the audience to justify it.

### Load testing

- **What:** k6 or hey runs against staging to validate pgx pool size, Railway plan tier, and bottleneck assumptions.
- **When:** Not needed at <50 businesses. Useful before opening signups beyond a controlled cohort.

### Uptime monitoring upgrade

- **What:** Move from free UptimeRobot (1 check, 5min interval) to BetterStack or paid UptimeRobot (multiple checks, 1min interval, on-call routing).
- **When:** When founder is no longer the only on-call, or when SLAs to customers require sub-5-minute detection.

---

## Auth / security

### Email verification on signup

- **What:** Send a verification code/link before allowing the user to log in.
- **Why deferred:** 5 hand-vetted businesses don't need it. The original WhatsApp-based design moved with WhatsApp itself (above).
- **Options when re-prioritizing:**
  - **WhatsApp OTP** — restore the original `/auth/verify-email` + `/auth/resend-code` design. Couples to WhatsApp infrastructure.
  - **Email OTP** via Resend — cheaper, no Meta approval, good fallback if WhatsApp drags. Frontend reuses the same `useVerifyEmail` hook with a switched delivery channel.
- **Required work:** depends on path. Email OTP is ~half a day; WhatsApp OTP requires Twilio integration above.

### Refresh token rotation with `jti` tracking

- **What:** Per `TODO-security.md` item 5, rotate refresh tokens on each use. Store `jti` in a `refresh_tokens` table; mark used; issue a new pair.
- **Why deferred:** Plain JWT refresh works for MVP. This is hardening against stolen-token replay.
- **When:** Before any non-trusted user gets API access — i.e. when public signup opens beyond hand-vetted businesses.

---

## Infrastructure improvements

### Switch frontend hosting from Cloudflare Pages to Vercel (or vice-versa)

- **Not planned.** Pages is the chosen MVP path for vendor consolidation and cost.
- **Trigger to revisit:** if you adopt Next.js or need server-side rendering. Vite SPA gives no advantage to Vercel.

### Switch Postgres from Railway-bundled to Neon

- **Not planned for MVP.** Bundled Railway Postgres is simpler and cheaper at 5 businesses.
- **Trigger to revisit:** when you want database branching for staging/migrations workflow, or Railway compute costs spike noticeably from DB workload.
- **Required work:** Neon project, region match with Railway backend, `pg_dump` from Railway → restore to Neon, swap `DATABASE_URL`.

### CI deploy-on-merge instead of Railway watching `main`

- **Not planned.** Railway's auto-deploy from `main` is fine for MVP.
- **Trigger to revisit:** when you want pre-deploy gates (e.g. require migrations to dry-run successfully) or per-PR preview deploys.
