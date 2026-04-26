CREATE INDEX IF NOT EXISTS idx_appointments_user_unseen
    ON appointments (user_id, created_at DESC)
    WHERE seen_at IS NULL;
