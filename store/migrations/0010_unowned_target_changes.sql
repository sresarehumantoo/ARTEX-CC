-- max_unowned_seconds is the longest a change sat unrestored with no live
-- claim on its intent, recorded when that window closes (a claim, an
-- adoption or a restore), so the report can show it after the fact.
ALTER TABLE target_changes
    ADD COLUMN IF NOT EXISTS max_unowned_seconds INT NOT NULL DEFAULT 0;

-- claim_renewed_at is when the holder last claimed or renewed the intent
-- itself. Renewal from proxy traffic never moves it and is capped at two
-- leases past it (one lease past when the claim would otherwise have
-- expired), so a dead holder's claim still lapses.
ALTER TABLE exploration_nodes
    ADD COLUMN IF NOT EXISTS claim_renewed_at TIMESTAMPTZ;

-- Live claims from before this migration get a renewal time derived from
-- the default 30 minute lease, never later than now, so traffic cannot
-- extend them without bound. Traffic renewal skips claims still without one.
UPDATE exploration_nodes
SET claim_renewed_at = LEAST(now(), claim_expires_at - interval '30 minutes')
WHERE kind = 'intent' AND state = 'running' AND claim_expires_at IS NOT NULL
  AND claim_renewed_at IS NULL;
