-- RFC 5545 SEQUENCE: bump on any change an iCalendar client needs to notice
-- (reschedule, cancel, metadata edit). Used by the ICS feed so subscribers'
-- Calendar apps update existing events in place instead of duplicating them.
ALTER TABLE appointments ADD COLUMN ical_sequence INT NOT NULL DEFAULT 0;
