-- Add agent update information to hosts table
ALTER TABLE hosts ADD COLUMN update_available INTEGER NOT NULL DEFAULT 0;
ALTER TABLE hosts ADD COLUMN latest_version TEXT DEFAULT '';
ALTER TABLE hosts ADD COLUMN update_url TEXT DEFAULT '';
