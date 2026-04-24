ALTER TABLE appointments
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'confirmed';

ALTER TABLE appointments
    ADD CONSTRAINT appointments_status_check
    CHECK (status IN ('pending', 'confirmed', 'cancelled', 'completed'));

CREATE INDEX idx_appointments_user_start ON appointments (user_id, start_time);
