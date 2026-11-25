ALTER TABLE findings ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;
UPDATE findings SET updated_at = created_at WHERE updated_at IS NULL;
ALTER TABLE findings
    ALTER COLUMN updated_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET NOT NULL;
DROP TRIGGER IF EXISTS trg_findings_upd ON findings;
CREATE TRIGGER trg_findings_upd BEFORE UPDATE ON findings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
