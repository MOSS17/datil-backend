-- Phase 6 pivot: Apple CalDAV → ICS subscription feed.
-- Rows written under the old apple-via-caldav shape (plaintext email +
-- app-specific password in access_token) are scrubbed rather than migrated;
-- ICS has different semantics and no existing rows should be preserved.
DELETE FROM calendar_integrations WHERE provider = 'apple';

-- ICS integrations have no OAuth tokens. Relax the NOT NULL on access_token
-- so a provider='ics' row can carry only a feed_token.
ALTER TABLE calendar_integrations ALTER COLUMN access_token DROP NOT NULL;

-- Per-user feed token. Partial unique index keeps the uniqueness guarantee
-- over non-null values while letting OAuth rows store NULL freely.
ALTER TABLE calendar_integrations ADD COLUMN feed_token TEXT;
CREATE UNIQUE INDEX calendar_integrations_feed_token_key
    ON calendar_integrations (feed_token)
    WHERE feed_token IS NOT NULL;

-- Drop apple_event_uid: it was added in migration 18 for the abandoned
-- Apple-via-CalDAV push path. The ICS feed is pull-based and doesn't need
-- an external event id on the appointment row.
ALTER TABLE appointments DROP COLUMN IF EXISTS apple_event_uid;
