-- Add category column to sources table
ALTER TABLE sources ADD COLUMN category text NOT NULL DEFAULT 'General';
