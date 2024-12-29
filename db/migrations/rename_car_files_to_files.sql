-- Add deal_id column if it doesn't exist
DO $$ 
BEGIN 
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name = 'files' 
        AND column_name = 'deal_id'
    ) THEN
        ALTER TABLE files 
        ADD COLUMN deal_id UUID,
        ADD CONSTRAINT files_deal_id_fkey 
        FOREIGN KEY (deal_id) 
        REFERENCES deals(uuid);
    END IF;
END $$;
