package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type Finding struct {
	ID            int64           `json:"id"`
	TaskID        *int64          `json:"task_id,omitempty"`
	NodeID        *int64          `json:"node_id,omitempty"`
	VulnClass     string          `json:"vulnclass"`
	Name          string          `json:"name"`
	Severity      string          `json:"severity"`
	Summary       string          `json:"summary"`
	Evidence      string          `json:"evidence"`
	Worker        string          `json:"worker"`
	AssetIDs      json.RawMessage `json:"asset_ids"`
	Status        string          `json:"status"`
	Report        string          `json:"report"`
	MergedInto    *int64          `json:"merged_into,omitempty"`
	Dismissed     bool            `json:"dismissed"`
	DismissReason string          `json:"dismiss_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

const findingSelectCols = `id, task_id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report, merged_into, dismissed, dismiss_reason, created_at`

func scanFinding(row interface{ Scan(...any) error }) (Finding, error) {
	var f Finding
	err := row.Scan(&f.ID, &f.TaskID, &f.NodeID, &f.VulnClass, &f.Name,
		&f.Severity, &f.Summary, &f.Evidence, &f.Worker, &f.AssetIDs,
		&f.Status, &f.Report, &f.MergedInto, &f.Dismissed, &f.DismissReason, &f.CreatedAt)
	return f, err
}

func (s *Store) AddFinding(ctx context.Context, f tools.FindingInput) (findingID, nodeID int64, err error) {
	assetIDs := "[]"
	if len(f.AssetIDs) > 0 {
		raw, _ := json.Marshal(f.AssetIDs)
		assetIDs = string(raw)
	}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	if err = tx.QueryRowContext(ctx, `
		INSERT INTO findings (task_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, '')
		RETURNING id`,
		nullID(f.TaskID), f.VulnClass, f.Name, f.Severity, f.Summary,
		f.Evidence, f.Worker, assetIDs, FindingStatusPending,
	).Scan(&findingID); err != nil {
		return 0, 0, err
	}

	payload, _ := json.Marshal(map[string]any{
		"finding_id": findingID,
		"vulnclass":  f.VulnClass,
		"name":       f.Name,
		"severity":   f.Severity,
		"summary":    f.Summary,
	})
	if err = tx.QueryRowContext(ctx, `
		INSERT INTO exploration_nodes (exploration_id, kind, payload, priority, state, origin)
		VALUES ($1, $2, $3, 5, 'confirmed', 'worker')
		RETURNING id`,
		f.ExplorationID, KindFinding, string(payload),
	).Scan(&nodeID); err != nil {
		return 0, 0, fmt.Errorf("add finding node: %w", err)
	}
	for _, a := range f.AssetIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO exploration_anchors (node_id, asset_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			nodeID, a); err != nil {
			return 0, 0, fmt.Errorf("anchor finding node: %w", err)
		}
	}
	if f.IntentID > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO exploration_edges (exploration_id, src_id, rel, dst_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
			f.ExplorationID, f.IntentID, RelYields, nodeID); err != nil {
			return 0, 0, fmt.Errorf("link intent to finding: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE findings SET node_id = $1 WHERE id = $2`, nodeID, findingID); err != nil {
		return 0, 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return findingID, nodeID, nil
}

func (s *Store) ListFindings(ctx context.Context, explorationID int64) ([]tools.Finding, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT f.id, f.node_id, f.vulnclass, f.severity, f.summary, COALESCE(f.status,''),
		       f.merged_into, f.dismissed, f.dismiss_reason
		FROM findings f
		JOIN tasks t ON t.id = f.task_id
		WHERE t.exploration_id = $1
		ORDER BY f.created_at DESC`, explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Finding
	for rows.Next() {
		var f tools.Finding
		var nodeID, mergedInto sql.NullInt64
		if err := rows.Scan(&f.ID, &nodeID, &f.VulnClass, &f.Severity, &f.Summary, &f.Status,
			&mergedInto, &f.Dismissed, &f.DismissReason); err != nil {
			return nil, err
		}
		if nodeID.Valid {
			f.NodeID = nodeID.Int64
		}
		if mergedInto.Valid {
			f.MergedInto = mergedInto.Int64
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) FindingExists(ctx context.Context, findingID int64) (bool, error) {
	var exists bool
	err := s.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM findings WHERE id = $1)", findingID).Scan(&exists)
	return exists, err
}

func (s *Store) GetFinding(id int64) (*Finding, error) {
	f, err := scanFinding(s.QueryRow("SELECT "+findingSelectCols+" FROM findings WHERE id = $1", id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) SetFindingStatus(id int64, status string) error {
	_, err := s.Exec("UPDATE findings SET status = $1 WHERE id = $2", status, id)
	return err
}

func (s *Store) ListFindingsForReport(ctx context.Context, explorationID int64) ([]tools.ReportFinding, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT f.id, f.vulnclass, COALESCE(f.name,''), f.severity, f.summary, COALESCE(f.evidence,''), COALESCE(f.status,'')
		FROM findings f
		JOIN tasks t ON t.id = f.task_id
		WHERE t.exploration_id = $1
		  AND f.dismissed = false
		  AND f.merged_into IS NULL
		ORDER BY f.created_at DESC`, explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.ReportFinding
	for rows.Next() {
		var f tools.ReportFinding
		if err := rows.Scan(&f.ID, &f.VulnClass, &f.Name, &f.Severity, &f.Summary, &f.Evidence, &f.Status); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) BindFindingTraffic(ctx context.Context, findingID int64, snapshotID, role, note string, position int) error {
	_, err := s.ExecContext(ctx, `
		INSERT INTO finding_traffic_bindings (finding_id, snapshot_id, role, note, position)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (finding_id, snapshot_id) DO UPDATE
		SET role = EXCLUDED.role, note = EXCLUDED.note, position = EXCLUDED.position`,
		findingID, snapshotID, role, note, position)
	return err
}

func (s *Store) ListFindingTraffic(ctx context.Context, findingID int64) ([]tools.TrafficBinding, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT id, finding_id, snapshot_id, role, note, position
		FROM finding_traffic_bindings
		WHERE finding_id = $1
		ORDER BY position, id`, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.TrafficBinding
	for rows.Next() {
		var b tools.TrafficBinding
		if err := rows.Scan(&b.ID, &b.FindingID, &b.SnapshotID, &b.Role, &b.Note, &b.Position); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) FindingTrafficCount(ctx context.Context, findingID int64) (int, error) {
	var count int
	err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM finding_traffic_bindings WHERE finding_id = $1`, findingID).Scan(&count)
	return count, err
}

// FindPotentialDuplicates returns the live findings of an exploration that may
// describe the same issue as a new finding. It is advisory, so it errs on the
// side of listing a candidate rather than missing one.
func (s *Store) FindPotentialDuplicates(ctx context.Context, explorationID int64, vulnclass, severity string, assetIDs []int64) ([]tools.Finding, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT f.id, f.vulnclass, f.severity, f.summary, f.asset_ids
		FROM findings f
		JOIN tasks t ON t.id = f.task_id
		WHERE t.exploration_id = $1
		  AND f.dismissed = false
		  AND f.merged_into IS NULL
		ORDER BY f.created_at DESC`, explorationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Finding
	for rows.Next() {
		var f tools.Finding
		var rawAssets []byte
		if err := rows.Scan(&f.ID, &f.VulnClass, &f.Severity, &f.Summary, &rawAssets); err != nil {
			return nil, err
		}
		var assets []int64
		_ = json.Unmarshal(rawAssets, &assets)
		if possibleDuplicate(vulnclass, severity, assetIDs, f.VulnClass, f.Severity, assets) {
			out = append(out, f)
		}
	}
	return out, rows.Err()
}

// possibleDuplicate matches on the normalized vulnclass when the assets
// overlap or either side names none, and on severity alone only when the
// assets overlap.
func possibleDuplicate(vulnclass, severity string, assets []int64, otherClass, otherSeverity string, otherAssets []int64) bool {
	overlap := false
	for _, a := range assets {
		for _, b := range otherAssets {
			if a == b {
				overlap = true
			}
		}
	}
	vc := tools.NormalizeVulnClass(vulnclass)
	if vc != "" && vc == tools.NormalizeVulnClass(otherClass) {
		return overlap || len(assets) == 0 || len(otherAssets) == 0
	}
	return overlap && severity == otherSeverity
}

func lockFindingTx(ctx context.Context, tx *sql.Tx, id int64) (*Finding, error) {
	var f Finding
	err := tx.QueryRowContext(ctx,
		"SELECT "+findingSelectCols+" FROM findings WHERE id = $1 FOR UPDATE", id,
	).Scan(&f.ID, &f.TaskID, &f.NodeID, &f.VulnClass, &f.Name,
		&f.Severity, &f.Summary, &f.Evidence, &f.Worker, &f.AssetIDs,
		&f.Status, &f.Report, &f.MergedInto, &f.Dismissed, &f.DismissReason, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// MergeFindings folds the secondaries into the live root of primaryID, the
// finding reached by following merged_into. Secondaries that are already
// merged, dismissed or the root itself are skipped and reported.
func (s *Store) MergeFindings(ctx context.Context, primaryID int64, secondaryIDs []int64) (*tools.MergedFinding, error) {
	for range mergeAttempts - 1 {
		out, err := s.mergeFindings(ctx, primaryID, secondaryIDs)
		if !errors.Is(err, errMergeChainChanged) {
			return out, err
		}
	}
	out, err := s.mergeFindings(ctx, primaryID, secondaryIDs)
	if errors.Is(err, errMergeChainChanged) {
		return nil, fmt.Errorf("merge chain of finding %d kept changing during %d attempts; retry the merge", primaryID, mergeAttempts)
	}
	return out, err
}

const mergeAttempts = 5

var errMergeChainChanged = errors.New("merge chain changed")

// mergeChain returns the merged_into chain above id, read without locks. It
// stops at a missing finding or a repeated id, leaving those errors to the
// locked walk.
func mergeChain(ctx context.Context, tx *sql.Tx, id int64) ([]int64, error) {
	var chain []int64
	seen := map[int64]bool{id: true}
	for {
		var next sql.NullInt64
		err := tx.QueryRowContext(ctx, "SELECT merged_into FROM findings WHERE id = $1", id).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (!next.Valid || seen[next.Int64])) {
			return chain, nil
		}
		if err != nil {
			return nil, fmt.Errorf("merge chain of finding %d: %w", id, err)
		}
		id = next.Int64
		seen[id] = true
		chain = append(chain, id)
	}
}

func (s *Store) mergeFindings(ctx context.Context, primaryID int64, secondaryIDs []int64) (*tools.MergedFinding, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	chain, err := mergeChain(ctx, tx, primaryID)
	if err != nil {
		return nil, err
	}

	// Locking the requested findings and the primary's chain in one id order
	// serializes concurrent merges over them without deadlocking, so two
	// opposite merges cannot both commit and leave neither live.
	ids := append([]int64{primaryID}, secondaryIDs...)
	ids = append(ids, chain...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	locked := make(map[int64]*Finding, len(ids))
	for _, id := range ids {
		f, err := lockFindingTx(ctx, tx, id)
		if err != nil {
			if id == primaryID {
				return nil, fmt.Errorf("primary finding %d: %w", id, err)
			}
			if slices.Contains(secondaryIDs, id) {
				return nil, fmt.Errorf("secondary finding %d: %w", id, err)
			}
			return nil, fmt.Errorf("finding %d (merge target of %d): %w", id, primaryID, err)
		}
		locked[id] = f
	}

	primary := locked[primaryID]
	seen := map[int64]bool{primaryID: true}
	for primary.MergedInto != nil {
		next := *primary.MergedInto
		if seen[next] {
			return nil, fmt.Errorf("finding %d has a merged_into cycle", primaryID)
		}
		seen[next] = true
		if locked[next] == nil {
			return nil, errMergeChainChanged
		}
		primary = locked[next]
	}
	if primary.Dismissed {
		if primary.ID == primaryID {
			return nil, fmt.Errorf("primary finding %d is dismissed; pick a live finding as primary_id", primaryID)
		}
		return nil, fmt.Errorf("primary finding %d was merged into finding %d, which is dismissed; pick a live finding as primary_id", primaryID, primary.ID)
	}
	rootID := primary.ID
	primaryLive := primary.NodeID != nil

	var primaryAssets []int64
	_ = json.Unmarshal(primary.AssetIDs, &primaryAssets)
	assetSet := map[int64]bool{}
	for _, a := range primaryAssets {
		assetSet[a] = true
	}

	var evidenceParts []string
	if primary.Evidence != "" {
		evidenceParts = append(evidenceParts, primary.Evidence)
	}

	var skipped []tools.SkippedFinding
	var merging []int64
	done := map[int64]bool{}
	for _, secID := range secondaryIDs {
		if secID == primaryID || done[secID] {
			continue
		}
		done[secID] = true
		sec := locked[secID]
		switch {
		case secID == rootID:
			skipped = append(skipped, tools.SkippedFinding{ID: secID, Reason: "is the resolved primary"})
		case sec.MergedInto != nil:
			skipped = append(skipped, tools.SkippedFinding{ID: secID, Reason: fmt.Sprintf("already merged into finding %d", *sec.MergedInto)})
		case sec.Dismissed:
			skipped = append(skipped, tools.SkippedFinding{ID: secID, Reason: "dismissed"})
		default:
			merging = append(merging, secID)
		}
	}
	if err := lockMergeNodesTx(ctx, tx, merging); err != nil {
		return nil, err
	}

	for _, secID := range merging {
		sec := locked[secID]
		var secAssets []int64
		_ = json.Unmarshal(sec.AssetIDs, &secAssets)
		for _, a := range secAssets {
			if !assetSet[a] {
				assetSet[a] = true
				primaryAssets = append(primaryAssets, a)
			}
		}

		if sec.Evidence != "" {
			evidenceParts = append(evidenceParts, fmt.Sprintf("[merged from #%d]\n%s", secID, sec.Evidence))
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE findings SET merged_into = $1, status = $2 WHERE id = $3",
			rootID, FindingStatusDuplicate, secID)
		if err != nil {
			return nil, fmt.Errorf("mark secondary %d: %w", secID, err)
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE exploration_nodes SET state = 'dismissed'
			 WHERE id = (SELECT node_id FROM findings WHERE id = $1) AND kind = 'finding'`, secID)
		if err != nil {
			return nil, fmt.Errorf("dismiss secondary %d node: %w", secID, err)
		}
		if primaryLive {
			if err := moveGoalProofsTx(ctx, tx, secID, *primary.NodeID); err != nil {
				return nil, err
			}
		}
		if err := revokeGoalProofsTx(ctx, tx, secID); err != nil {
			return nil, err
		}
	}

	mergedAssetJSON, _ := json.Marshal(primaryAssets)
	mergedEvidence := strings.Join(evidenceParts, "\n\n")

	_, err = tx.ExecContext(ctx,
		"UPDATE findings SET asset_ids = $1, evidence = $2 WHERE id = $3",
		string(mergedAssetJSON), mergedEvidence, rootID)
	if err != nil {
		return nil, fmt.Errorf("update primary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	out := &tools.MergedFinding{
		ID:        rootID,
		VulnClass: primary.VulnClass,
		Name:      primary.Name,
		Severity:  primary.Severity,
		Summary:   primary.Summary,
		Evidence:  mergedEvidence,
		AssetIDs:  primaryAssets,
		Status:    primary.Status,
		Skipped:   skipped,
	}
	if rootID != primaryID {
		out.RequestedPrimaryID = primaryID
	}
	return out, nil
}

func (s *Store) DismissFinding(ctx context.Context, findingID int64, reason string) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		"UPDATE findings SET dismissed = true, dismiss_reason = $1, status = $2 WHERE id = $3",
		reason, FindingStatusFalsePositive, findingID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("finding %d not found", findingID)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE exploration_nodes SET state = 'dismissed'
		 WHERE id = (SELECT node_id FROM findings WHERE id = $1) AND kind = 'finding'`, findingID); err != nil {
		return err
	}
	if err := revokeGoalProofsTx(ctx, tx, findingID); err != nil {
		return err
	}
	return tx.Commit()
}

// lockMergeNodesTx locks the nodes of the merging findings, then every goal
// they prove, each set in one statement in id order. ProveGoal and
// DismissFinding also lock an evidence node before its goals, so taking all
// of them up front keeps every merge, proof and dismissal in one lock order.
func lockMergeNodesTx(ctx context.Context, tx *sql.Tx, findingIDs []int64) error {
	if len(findingIDs) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		SELECT n.id FROM exploration_nodes n JOIN findings f ON f.node_id = n.id
		WHERE f.id = ANY($1)
		ORDER BY n.id FOR NO KEY UPDATE OF n`,
		findingIDs); err != nil {
		return fmt.Errorf("lock nodes of merged findings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		SELECT g.id FROM exploration_nodes g
		WHERE g.kind = $3 AND g.id IN (
			SELECT e.dst_id FROM exploration_edges e JOIN findings f ON f.node_id = e.src_id
			WHERE f.id = ANY($1) AND e.rel = $2)
		ORDER BY g.id FOR NO KEY UPDATE`,
		findingIDs, RelProves, KindGoal); err != nil {
		return fmt.Errorf("lock goals proved by merged findings: %w", err)
	}
	return nil
}

// moveGoalProofsTx copies the proves edges of a finding's node onto toNode.
func moveGoalProofsTx(ctx context.Context, tx *sql.Tx, findingID, toNode int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO exploration_edges (exploration_id, src_id, rel, dst_id)
		SELECT e.exploration_id, $2, e.rel, e.dst_id
		FROM exploration_edges e JOIN findings f ON f.node_id = e.src_id
		WHERE f.id = $1 AND e.rel = $3 AND e.dst_id <> $2
		ON CONFLICT (exploration_id, src_id, rel, dst_id) DO NOTHING`,
		findingID, toNode, RelProves)
	if err != nil {
		return fmt.Errorf("move goal proofs of finding %d: %w", findingID, err)
	}
	return nil
}

// revokeGoalProofsTx removes the proves edges of a finding's node and reopens
// each met goal it proved that has no other proof from a live node.
func revokeGoalProofsTx(ctx context.Context, tx *sql.Tx, findingID int64) error {
	// Locking the goals first serializes concurrent revocations of different
	// proofs of one goal: the later one waits, then its fresh READ COMMITTED
	// snapshot sees the earlier one's deleted edge and dismissed node.
	if _, err := tx.ExecContext(ctx, `
		SELECT g.id FROM exploration_nodes g
		WHERE g.kind = $3 AND g.id IN (
			SELECT e.dst_id FROM exploration_edges e JOIN findings f ON f.node_id = e.src_id
			WHERE f.id = $1 AND e.rel = $2)
		ORDER BY g.id FOR NO KEY UPDATE`,
		findingID, RelProves, KindGoal); err != nil {
		return fmt.Errorf("lock goals proved by finding %d: %w", findingID, err)
	}
	_, err := tx.ExecContext(ctx, `
		WITH revoked AS (
			DELETE FROM exploration_edges e USING findings f
			WHERE f.id = $1 AND e.src_id = f.node_id AND e.rel = $2
			RETURNING e.src_id, e.dst_id
		)
		UPDATE exploration_nodes g SET state = 'open'
		FROM revoked r
		WHERE g.id = r.dst_id AND g.kind = $3 AND g.state = 'met'
		  AND NOT EXISTS (
			SELECT 1 FROM exploration_edges pe
			JOIN exploration_nodes src ON src.id = pe.src_id
			WHERE pe.dst_id = g.id AND pe.rel = $2
			  AND pe.src_id <> r.src_id AND src.state <> 'dismissed')`,
		findingID, RelProves, KindGoal)
	if err != nil {
		return fmt.Errorf("revoke goal proofs of finding %d: %w", findingID, err)
	}
	return nil
}
