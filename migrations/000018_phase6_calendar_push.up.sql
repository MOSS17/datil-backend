ALTER TABLE calendar_integrations ADD COLUMN account_email VARCHAR(255);

ALTER TABLE appointments ADD COLUMN google_event_id VARCHAR(1024);
ALTER TABLE appointments ADD COLUMN apple_event_uid VARCHAR(1024);
