-- Add AWX-style targeting fields to job_templates
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS limit_hosts TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS verbosity INTEGER NOT NULL DEFAULT 0;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS forks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS job_tags TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS skip_tags TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS extra_vars JSONB NOT NULL DEFAULT '{}';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS ask_inventory_on_launch BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS ask_limit_on_launch BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS ask_verbosity_on_launch BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS ask_variables_on_launch BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS ask_credential_on_launch BOOLEAN NOT NULL DEFAULT false;

-- Add description to inventory_hosts and inventory_groups
ALTER TABLE inventory_hosts ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE inventory_groups ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';

-- Add description to inventories
ALTER TABLE inventories ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
