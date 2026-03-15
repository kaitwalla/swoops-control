-- Add working_directory column to sessions table
-- This allows sessions to use custom working directories instead of worktrees
ALTER TABLE sessions ADD COLUMN working_directory TEXT DEFAULT '';
