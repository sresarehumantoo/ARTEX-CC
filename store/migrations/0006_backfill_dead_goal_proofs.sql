WITH RECURSIVE chain AS (
    SELECT f.id AS start_id, f.merged_into AS next_id, ARRAY[f.id] AS path
    FROM findings f
    WHERE f.merged_into IS NOT NULL
    UNION ALL
    SELECT c.start_id, f.merged_into, c.path || f.id
    FROM chain c JOIN findings f ON f.id = c.next_id
    WHERE f.id <> ALL (c.path)
),
roots AS (
    SELECT c.start_id, c.path[array_length(c.path, 1)] AS root_id
    FROM chain c
    WHERE c.next_id IS NULL
)
INSERT INTO exploration_edges (exploration_id, src_id, rel, dst_id)
SELECT e.exploration_id, rf.node_id, e.rel, e.dst_id
FROM exploration_edges e
JOIN findings sf ON sf.node_id = e.src_id
JOIN roots r ON r.start_id = sf.id
JOIN findings rf ON rf.id = r.root_id
JOIN exploration_nodes rn ON rn.id = rf.node_id
WHERE e.rel = 'proves'
  AND NOT rf.dismissed
  AND rn.state <> 'dismissed'
  AND e.dst_id <> rf.node_id
ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING;

WITH revoked AS (
    DELETE FROM exploration_edges e USING findings f
    WHERE e.src_id = f.node_id AND e.rel = 'proves'
      AND (f.dismissed OR f.merged_into IS NOT NULL)
    RETURNING e.dst_id
)
UPDATE exploration_nodes g SET state = 'open'
WHERE g.id IN (SELECT dst_id FROM revoked)
  AND g.kind = 'goal' AND g.state = 'met'
  AND NOT EXISTS (
    SELECT 1 FROM exploration_edges pe
    JOIN exploration_nodes src ON src.id = pe.src_id
    WHERE pe.dst_id = g.id AND pe.rel = 'proves'
      AND src.state <> 'dismissed'
      AND NOT EXISTS (
        SELECT 1 FROM findings df
        WHERE df.node_id = pe.src_id AND (df.dismissed OR df.merged_into IS NOT NULL)));
