-- max_unowned_seconds is the longest a change sat unrestored with no live
-- claim on its intent, recorded when that window closes (a claim, an
-- adoption or a restore), so the report can show it after the fact.
ALTER TABLE target_changes
    ADD COLUMN IF NOT EXISTS max_unowned_seconds INT NOT NULL DEFAULT 0;

-- claim_renewed_at is when the holder last claimed or renewed the intent
-- itself. Renewal from proxy traffic never moves it, and is capped one lease
-- past it, so a dead holder's claim still lapses.
ALTER TABLE exploration_nodes
    ADD COLUMN IF NOT EXISTS claim_renewed_at TIMESTAMPTZ;
