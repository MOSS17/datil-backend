DROP INDEX IF EXISTS idx_appointments_user_start;
ALTER TABLE appointments DROP CONSTRAINT IF EXISTS appointments_status_check;
ALTER TABLE appointments DROP COLUMN IF EXISTS status;
