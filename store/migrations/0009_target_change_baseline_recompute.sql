-- 0005 keyed POSIX paths case-insensitively, so a change on /srv/app.conf
-- could inherit the baseline of /srv/App.conf. Rebuild baselines the way
-- RecordTargetChange assigns them: a restore closes every live change on a
-- resource at once, so the changes on one resource key sharing a restored_at
-- (NULL for live) form one chain whose first change is the baseline.
WITH chains AS (
    SELECT id,
           first_value(id) OVER w AS root_id,
           first_value(original_fingerprint) OVER w AS root_fingerprint
    FROM target_changes
    WINDOW w AS (
        PARTITION BY exploration_id, lower(btrim(target)), target_change_resource_key(resource), restored_at
        ORDER BY id
    )
)
UPDATE target_changes tc
SET baseline_change_id = c.root_id, baseline = c.root_fingerprint
FROM chains c
WHERE tc.id = c.id
  AND (tc.baseline_change_id IS DISTINCT FROM c.root_id OR tc.baseline IS DISTINCT FROM c.root_fingerprint);
