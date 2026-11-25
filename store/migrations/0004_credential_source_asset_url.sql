ALTER TABLE task_credentials DROP CONSTRAINT IF EXISTS task_credentials_source_check;
ALTER TABLE task_credentials ADD CONSTRAINT task_credentials_source_check
    CHECK (source IN ('user','agent','asset_url'));
