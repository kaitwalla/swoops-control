-- Add agent authentication token to hosts table
ALTER TABLE hosts ADD COLUMN agent_auth_token TEXT NOT NULL DEFAULT '';

-- Generate random tokens for existing hosts (they'll need to be regenerated properly)
-- In production, existing hosts should get new tokens via a secure process
