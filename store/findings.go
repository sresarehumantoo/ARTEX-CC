package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

type Finding struct {
	ID        int64           `json:"id"`
	TaskID    *int64          `json:"task_id,omitempty"`
	NodeID    *int64          `json:"node_id,omitempty"`
	VulnClass string          `json:"vulnclass"`
	Name      string          `json:"name"`
	Severity  string          `json:"severity"`
	Summary   string          `json:"summary"`
	Evidence  string          `json:"evidence"`
	Worker    string          `json:"worker"`
	AssetIDs  json.RawMessage `json:"asset_ids"`
	Status    string          `json:"status"`
	Report        string          `json:"report"`
	MergedInto    *int64          `json:"merged_into,omitempty"`
	Dismissed     bool            `json:"dismissed"`
	DismissReason string          `json:"dismiss_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type FindingFilter struct {
	TaskID   int64
	Status   string
	Severity string
	Limit    int
	Offset   int
}

const findingSelectCols = `id, task_id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report, merged_into, dismissed, dismiss_reason, created_at`

func scanFinding(row interface{ Scan(...any) error }) (Finding, error) {
	var f Finding
	err := row.Scan(&f.ID, &f.TaskID, &f.NodeID, &f.VulnClass, &f.Name,
		&f.Severity, &f.Summary, &f.Evidence, &f.Worker, &f.AssetIDs,
		&f.Status, &f.Report, &f.MergedInto, &f.Dismissed, &f.DismissReason, &f.CreatedAt)
	return f, err
}

func scanFindings(rows *sql.Rows) ([]Finding, error) {
	var out []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) AddFinding(ctx context.Context, f tools.FindingInput) (int64, error) {
	assetIDs := "[]"
	if len(f.AssetIDs) > 0 {
		raw, _ := json.Marshal(f.AssetIDs)
		assetIDs = string(raw)
	}
	var id int64
	err := s.QueryRowContext(ctx, `
		INSERT INTO findings (task_id, node_id, vulnclass, name, severity, summary, evidence, worker, asset_ids, status, report)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, '')
		RETURNING id`,
		nullID(f.TaskID), nullID(f.IntentID), f.VulnClass, f.Name, f.Severity, f.Summary,
		f.Evidence, f.Worker, assetIDs, FindingStatusPending,
	).Scan(&id)
	return id, err
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

func (s *Store) listFindingsInternal(filter FindingFilter) ([]Finding, int, error) {
	var conditions []string
	var args []any
	idx := 1
	addCond := func(clause string, val any) {
		args = append(args, val)
		conditions = append(conditions, fmt.Sprintf(clause, idx))
		idx++
	}

	if filter.TaskID > 0 {
		addCond("task_id = $%d", filter.TaskID)
	}
	if filter.Status != "" {
		addCond("status = $%d", filter.Status)
	}
	if filter.Severity != "" {
		addCond("severity = $%d", filter.Severity)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.QueryRow("SELECT COUNT(*) FROM findings"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf("SELECT %s FROM findings%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		findingSelectCols, where, idx, idx+1)

	rows, err := s.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	findings, err := scanFindings(rows)
	if err != nil {
		return nil, 0, err
	}
	return findings, total, nil
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

func (s *Store) FindPotentialDuplicates(ctx context.Context, severity string, assetIDs []int64) ([]int64, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	assetJSON, err := json.Marshal(assetIDs)
	if err != nil {
		return nil, err
	}
	rows, err := s.QueryContext(ctx, `
		SELECT id FROM findings
		WHERE severity = $1
		  AND dismissed = false
		  AND merged_into IS NULL
		  AND asset_ids != '[]'::jsonb
		  AND asset_ids ?| (
			SELECT array_agg(elem::text)
			FROM jsonb_array_elements($2::jsonb) AS elem
		  )
		ORDER BY created_at DESC`, severity, string(assetJSON))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
	return nil
}
