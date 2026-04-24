ALTER TABLE appointments ADD COLUMN IF NOT EXISTS apple_event_uid VARCHAR(1024);

DROP INDEX IF EXISTS calendar_integrations_feed_token_key;
ALTER TABLE calendar_integrations DROP COLUMN IF EXISTS feed_token;

-- Any ICS rows without OAuth tokens would violate NOT NULL if we tried to
-- restore it as-is; drop them before reinstating the constraint.
DELETE FROM calendar_integrations WHERE access_token IS NULL;
ALTER TABLE calendar_integrations ALTER COLUMN access_token SET NOT NULL;
