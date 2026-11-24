package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (s *Store) LogActivity(ctx context.Context, explorationID int64, entry tools.ActivityEntry) (int64, error) {
	var id int64
	err := s.QueryRowContext(ctx, `
		INSERT INTO activity (exploration_id, node_id, worker, kind, tool, is_error, summary, detail)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		explorationID,
		entry.NodeID,
		entry.Worker,
		entry.Kind,
		entry.Tool,
		entry.IsError,
		entry.Summary,
		entry.Detail,
	).Scan(&id)
	return id, err
}

func (s *Store) ListActivity(ctx context.Context, explorationID int64, filter tools.ActivityFilter) ([]tools.Activity, error) {
	var conditions []string
	var args []any
	idx := 1

	addCond := func(clause string, val any) {
		args = append(args, val)
		conditions = append(conditions, fmt.Sprintf(clause, idx))
		idx++
	}

	addCond("exploration_id = $%d", explorationID)

	if filter.NodeID > 0 {
		addCond("node_id = $%d", filter.NodeID)
	}
	if filter.Kind != "" {
		addCond("kind = $%d", filter.Kind)
	}
	if filter.Since != "" {
		addCond("created_at >= $%d::timestamptz", filter.Since)
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	args = append(args, limit)
	q := fmt.Sprintf(`
		SELECT id, exploration_id, node_id, COALESCE(worker,''), COALESCE(kind,''),
		       COALESCE(tool,''), is_error, COALESCE(summary,''), COALESCE(detail,''),
		       created_at::text
		FROM activity%s
		ORDER BY id DESC
		LIMIT $%d`, where, idx)

	rows, err := s.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []tools.Activity
	for rows.Next() {
		var a tools.Activity
		var nodeID sql.NullInt64
		if err := rows.Scan(&a.ID, &a.ExplorationID, &nodeID, &a.Worker, &a.Kind,
			&a.Tool, &a.IsError, &a.Summary, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		if nodeID.Valid {
			a.NodeID = nodeID.Int64
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
