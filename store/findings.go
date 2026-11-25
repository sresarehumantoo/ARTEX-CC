package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func (s *Store) MergeFindings(ctx context.Context, primaryID int64, secondaryIDs []int64) (*tools.MergedFinding, error) {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var primary Finding
	err = tx.QueryRowContext(ctx,
		"SELECT "+findingSelectCols+" FROM findings WHERE id = $1 FOR UPDATE", primaryID,
	).Scan(&primary.ID, &primary.TaskID, &primary.NodeID, &primary.VulnClass, &primary.Name,
		&primary.Severity, &primary.Summary, &primary.Evidence, &primary.Worker, &primary.AssetIDs,
		&primary.Status, &primary.Report, &primary.MergedInto, &primary.Dismissed, &primary.DismissReason, &primary.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("primary finding %d: %w", primaryID, err)
	}

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

	for _, secID := range secondaryIDs {
		if secID == primaryID {
			continue
		}
		var sec Finding
		err = tx.QueryRowContext(ctx,
			"SELECT "+findingSelectCols+" FROM findings WHERE id = $1 FOR UPDATE", secID,
		).Scan(&sec.ID, &sec.TaskID, &sec.NodeID, &sec.VulnClass, &sec.Name,
			&sec.Severity, &sec.Summary, &sec.Evidence, &sec.Worker, &sec.AssetIDs,
			&sec.Status, &sec.Report, &sec.MergedInto, &sec.Dismissed, &sec.DismissReason, &sec.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("secondary finding %d: %w", secID, err)
		}

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
			primaryID, FindingStatusDuplicate, secID)
		if err != nil {
			return nil, fmt.Errorf("mark secondary %d: %w", secID, err)
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE exploration_nodes SET state = 'dismissed'
			 WHERE id = (SELECT node_id FROM findings WHERE id = $1) AND kind = 'finding'`, secID)
		if err != nil {
			return nil, fmt.Errorf("dismiss secondary %d node: %w", secID, err)
		}
	}

	mergedAssetJSON, _ := json.Marshal(primaryAssets)
	mergedEvidence := strings.Join(evidenceParts, "\n\n")

	_, err = tx.ExecContext(ctx,
		"UPDATE findings SET asset_ids = $1, evidence = $2 WHERE id = $3",
		string(mergedAssetJSON), mergedEvidence, primaryID)
	if err != nil {
		return nil, fmt.Errorf("update primary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &tools.MergedFinding{
		ID:        primaryID,
		VulnClass: primary.VulnClass,
		Name:      primary.Name,
		Severity:  primary.Severity,
		Summary:   primary.Summary,
		Evidence:  mergedEvidence,
		AssetIDs:  primaryAssets,
		Status:    primary.Status,
	}, nil
}

func (s *Store) DismissFinding(ctx context.Context, findingID int64, reason string) error {
	res, err := s.ExecContext(ctx,
		"UPDATE findings SET dismissed = true, dismiss_reason = $1, status = $2 WHERE id = $3",
		reason, FindingStatusFalsePositive, findingID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("finding %d not found", findingID)
	}
	_, err = s.ExecContext(ctx,
		`UPDATE exploration_nodes SET state = 'dismissed'
		 WHERE id = (SELECT node_id FROM findings WHERE id = $1) AND kind = 'finding'`, findingID)
	return err
}
