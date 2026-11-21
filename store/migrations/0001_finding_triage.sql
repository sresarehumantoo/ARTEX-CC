ALTER TABLE findings
    ADD COLUMN IF NOT EXISTS merged_into    BIGINT REFERENCES findings(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS dismissed       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS dismiss_reason  TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_findings_merged_into ON findings(merged_into) WHERE merged_into IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_findings_dismissed ON findings(dismissed) WHERE dismissed = true;
