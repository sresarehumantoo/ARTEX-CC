CREATE TABLE IF NOT EXISTS target_changes (
    id                   BIGSERIAL PRIMARY KEY,
    exploration_id       BIGINT NOT NULL REFERENCES explorations(id) ON DELETE CASCADE,
    target               TEXT NOT NULL,
    resource             TEXT NOT NULL,
    kind                 TEXT NOT NULL CHECK (kind IN ('modify','create','delete','other')),
    intent_id            BIGINT REFERENCES exploration_nodes(id) ON DELETE SET NULL,
    original_fingerprint TEXT NOT NULL,
    baseline             TEXT NOT NULL,
    baseline_change_id   BIGINT REFERENCES target_changes(id) ON DELETE SET NULL,
    backup               TEXT NOT NULL DEFAULT '',
    note                 TEXT NOT NULL DEFAULT '',
    recorded_by          TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    restored_at          TIMESTAMPTZ,
    restored_by          TEXT NOT NULL DEFAULT '',
    last_observed        TEXT NOT NULL DEFAULT '',
    restore_attempts     INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_target_changes_live
    ON target_changes(exploration_id, lower(btrim(target)), lower(btrim(resource)))
    WHERE restored_at IS NULL;
