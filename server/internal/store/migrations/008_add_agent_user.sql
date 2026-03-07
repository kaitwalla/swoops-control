-- Add agent_user column to hosts table to track which user the agent is running as
ALTER TABLE hosts ADD COLUMN agent_user TEXT DEFAULT '';
