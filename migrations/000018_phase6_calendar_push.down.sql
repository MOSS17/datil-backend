ALTER TABLE appointments DROP COLUMN IF EXISTS apple_event_uid;
ALTER TABLE appointments DROP COLUMN IF EXISTS google_event_id;

ALTER TABLE calendar_integrations DROP COLUMN IF EXISTS account_email;
