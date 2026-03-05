-- Add cert_downloaded column to track if client certificates have been downloaded
ALTER TABLE hosts ADD COLUMN cert_downloaded BOOLEAN NOT NULL DEFAULT 0;
