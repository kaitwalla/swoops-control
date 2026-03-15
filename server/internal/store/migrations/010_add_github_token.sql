-- Add github_token column to users table for GitHub API access
-- This allows users to authenticate with GitHub to list/create repos
ALTER TABLE users ADD COLUMN github_token TEXT DEFAULT '';
