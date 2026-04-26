# How Datil's costs scale — a non-technical rundown

**The short version:** until we have real customers, this thing costs about as much as a couple of streaming subscriptions. The bills only start mattering once WhatsApp messages start flying.

---

## What we actually pay for

Think of it like running a small restaurant. We have:

- **Rent** — the servers that run the app (Railway + the database). Fixed-ish; goes up in steps, not smoothly.
- **Utilities** — file storage for logos and photos (Cloudflare R2), and the website hosting (Vercel). Tiny. Stays tiny basically forever.
- **Per-customer ingredients** — every WhatsApp confirmation/reminder we send costs us a few cents (Twilio). This is the one that grows with usage.
- **Insurance** — error tracking, uptime monitoring, backups. Cheap to free until we're big.

The domain name (`datil.mx`) is ~$15/year. Round to zero.

---

## The three stages

### Stage 1 — "Just opened the doors" (0–5 paying businesses)
**~$10–15/month total.**
Everything's on the cheapest tier. WhatsApp costs are noise. We could run here for a year and barely notice.

### Stage 2 — "We have a real business" (20–50 paying businesses, ~2,000 appointments/month)
**~$60–100/month total.**
We bump the server up one tier so it doesn't hiccup under load. WhatsApp is now ~$30/mo — still small, but it's the first thing you can *see* on the bill.

### Stage 3 — "This is working" (100+ businesses, ~10,000 appointments/month)
**~$300–600/month total.**
Now WhatsApp is roughly half the bill. This is also where we start caring about details — like, do we send 1 reminder or 3? Because at this scale that decision is hundreds of dollars.

---

## What this means

A few things worth internalizing:

1. **It's not a flat $X/month — it's a staircase.** Costs don't creep up smoothly. They jump when we cross thresholds (server tier, database tier). Plan for the jump, not the average.
2. **WhatsApp is the variable cost.** Every appointment confirmation = a few cents. Every reminder = a few cents. If a business does 500 appointments/month and we send confirmation + reminder + follow-up, that's ~$20/mo *in messages alone* for that one customer. Pricing has to cover this.
3. **Marketing-style messages cost ~3× more than transactional ones.** "Don't forget your appointment" = cheap. "We have a promo this week!" = pricey. We should keep marketing nudges out of the core product cost.
4. **Storage is free in practice.** Photos and logos basically don't cost us anything, even at scale. Don't worry about that line.
5. **The first ~$50/month of error tracking and uptime monitoring isn't optional** — it's the difference between knowing a customer is broken and finding out from an angry email. Treat it as a fixed cost from day one.

---

## A useful gut-check rule

**If a business pays us less than ~$5/month, WhatsApp eats the margin.**

That's the floor for any pricing tier. Anything below that and we're paying Twilio to serve them. Useful number to keep in mind when the pricing conversation comes up.
