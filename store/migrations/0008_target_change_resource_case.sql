-- Windows paths and registry keys are case-insensitive, POSIX paths and
-- other resources are not, so only the former fold case.
CREATE OR REPLACE FUNCTION target_change_resource_key(resource TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE
        WHEN btrim(resource) ~* '^([a-z]:([\\/]|$)|hkey_|hk(lm|cu|cr|u|cc)([:\\/]|$))|\\'
            THEN lower(btrim(resource))
        ELSE btrim(resource)
    END
$$;

DROP INDEX IF EXISTS idx_target_changes_live;

CREATE INDEX IF NOT EXISTS idx_target_changes_live_key
    ON target_changes(exploration_id, lower(btrim(target)), target_change_resource_key(resource))
    WHERE restored_at IS NULL;
