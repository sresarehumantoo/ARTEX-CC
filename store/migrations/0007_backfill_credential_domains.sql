-- domain_key matches Go's DomainKey: trim Unicode white space, lowercase, drop one trailing dot.
CREATE TEMP TABLE credential_domain_keys ON COMMIT DROP AS
SELECT k.*,
       row_number() OVER (PARTITION BY task_id, domain_key, username ORDER BY updated_at DESC, id DESC) AS rank,
       count(*) OVER (PARTITION BY task_id, domain_key, username) AS group_size
FROM (
    SELECT id, task_id, username, password, note, created_at, updated_at,
           regexp_replace(lower(btrim(domain,
               chr(9) || chr(10) || chr(11) || chr(12) || chr(13) || chr(32) || chr(133) || chr(160) ||
               chr(5760) || chr(8192) || chr(8193) || chr(8194) || chr(8195) || chr(8196) || chr(8197) ||
               chr(8198) || chr(8199) || chr(8200) || chr(8201) || chr(8202) || chr(8232) || chr(8233) ||
               chr(8239) || chr(8287) || chr(12288))), '\.$', '') AS domain_key
    FROM task_credentials
) k;

-- The newest row of a colliding group survives with its own password and
-- source. An empty password there takes the newest non-empty one, copied
-- verbatim so enc: values stay encrypted, and every distinct note is kept.
UPDATE task_credentials c
SET password   = CASE WHEN c.password = '' THEN COALESCE(m.password, '') ELSE c.password END,
    note       = m.note,
    created_at = m.created_at
FROM (
    SELECT w.id,
           (SELECT k.password FROM credential_domain_keys k
             WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)
               AND k.password <> ''
             ORDER BY k.rank LIMIT 1) AS password,
           (SELECT COALESCE(string_agg(n.note, E'\n' ORDER BY n.first_rank), '')
              FROM (SELECT k.note, min(k.rank) AS first_rank FROM credential_domain_keys k
                     WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)
                       AND k.note <> ''
                     GROUP BY k.note) n) AS note,
           (SELECT min(k.created_at) FROM credential_domain_keys k
             WHERE (k.task_id, k.domain_key, k.username) = (w.task_id, w.domain_key, w.username)) AS created_at
    FROM credential_domain_keys w
    WHERE w.rank = 1 AND w.group_size > 1
) m
WHERE c.id = m.id;

DELETE FROM task_credentials c
USING credential_domain_keys k
WHERE c.id = k.id AND k.rank > 1;

UPDATE task_credentials c
SET domain = k.domain_key
FROM credential_domain_keys k
WHERE c.id = k.id AND c.domain <> k.domain_key;
