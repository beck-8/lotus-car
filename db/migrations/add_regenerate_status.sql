-- Add regenerate_status column to files table
ALTER TABLE files ADD COLUMN IF NOT EXISTS regenerate_status TEXT NOT NULL DEFAULT 'pending';

-- Add check constraint for regenerate_status values
ALTER TABLE files ADD CONSTRAINT check_regenerate_status CHECK (regenerate_status IN ('pending', 'success', 'failed'));
