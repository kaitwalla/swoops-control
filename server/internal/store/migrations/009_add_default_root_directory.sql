-- Add default_root_directory column to hosts table
-- This specifies the default directory where agent sessions will work
ALTER TABLE hosts ADD COLUMN default_root_directory TEXT DEFAULT '';
