package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type Task struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Goal          string    `json:"goal"`
	ExplorationID int64     `json:"exploration_id"`
	Status        string    `json:"status"`
	CompanyID     *int64    `json:"company_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (s *Store) CreateTask(ctx context.Context, name, description, goal string, companyID *int64) (*Task, error) {
	expID, err := s.CreateExploration(description, goal)
	if err != nil {
		return nil, err
	}

	var t Task
	err = s.QueryRowContext(ctx, `
		INSERT INTO tasks (name, description, goal, exploration_id, company_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, goal, exploration_id, status, company_id, created_at, updated_at`,
		name, description, goal, expID, companyID,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Goal, &t.ExplorationID, &t.Status, &t.CompanyID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := s.AddNode(ctx, expID, KindFact, map[string]any{"text": "task root"}, 0, StateOrigin, "system", nil); err != nil {
		return nil, err
	}

	return &t, nil
}

func (s *Store) GetTask(id int64) (*Task, error) {
	var t Task
	err := s.QueryRow(`
		SELECT id, name, description, goal, exploration_id, status, company_id, created_at, updated_at
		FROM tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Goal, &t.ExplorationID, &t.Status, &t.CompanyID, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListTasks() ([]Task, error) {
	rows, err := s.Query(`
		SELECT id, name, description, goal, exploration_id, status, company_id, created_at, updated_at
		FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Goal, &t.ExplorationID, &t.Status, &t.CompanyID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) SetTaskStatus(id int64, status string) error {
	_, err := s.Exec(`UPDATE tasks SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (s *Store) CompleteEngagement(ctx context.Context, taskID int64, summary string) error {
	t, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %d not found", taskID)
	}

	if _, err := s.ExecContext(ctx, `UPDATE tasks SET status = 'done' WHERE id = $1`, taskID); err != nil {
		return fmt.Errorf("update task status: %w", err)
	}
	if _, err := s.ExecContext(ctx, `UPDATE explorations SET status = 'achieved' WHERE id = $1`, t.ExplorationID); err != nil {
		return fmt.Errorf("update exploration status: %w", err)
	}

	if summary != "" {
		_, _ = s.ExecContext(ctx, `
			INSERT INTO activity (exploration_id, kind, summary)
			VALUES ($1, 'engagement_complete', $2)`,
			t.ExplorationID, summary)
	}

	return nil
}

func (s *Store) ListEngagements(ctx context.Context) ([]tools.EngagementSummary, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT t.id, t.goal, t.description, COALESCE(t.status,''), t.exploration_id,
		       t.created_at,
		       (SELECT COUNT(*) FROM exploration_nodes WHERE exploration_id = t.exploration_id),
		       (SELECT COUNT(*) FROM findings WHERE task_id = t.id),
		       (SELECT COUNT(*) FROM assets WHERE t.id = ANY(task_ids)),
		       (SELECT CASE WHEN COUNT(*) = 0 THEN false
		                    ELSE bool_and(state = 'met')
		               END
		        FROM exploration_nodes
		        WHERE exploration_id = t.exploration_id AND kind = 'goal')
		FROM tasks t ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.EngagementSummary
	for rows.Next() {
		var e tools.EngagementSummary
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.Goal, &e.Description, &e.Status, &e.ExplorationID,
			&createdAt, &e.NodeCount, &e.FindingCount, &e.AssetCount, &e.GoalsMet); err != nil {
			return nil, err
		}
		e.CreatedAt = createdAt.Format(time.RFC3339)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UpdateEngagement(ctx context.Context, taskID int64, goal, description *string) error {
	t, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %d not found", taskID)
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if goal != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET goal = $1, name = $1 WHERE id = $2`, *goal, taskID); err != nil {
			return fmt.Errorf("update task goal: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE explorations SET goal = $1 WHERE id = $2`, *goal, t.ExplorationID); err != nil {
			return fmt.Errorf("update exploration goal: %w", err)
		}
	}
	if description != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET description = $1 WHERE id = $2`, *description, taskID); err != nil {
			return fmt.Errorf("update task description: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE explorations SET description = $1 WHERE id = $2`, *description, t.ExplorationID); err != nil {
			return fmt.Errorf("update exploration description: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteEngagement(ctx context.Context, taskID int64) error {
	t, err := s.GetTask(taskID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %d not found", taskID)
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM findings WHERE task_id = $1`, taskID); err != nil {
		return fmt.Errorf("delete findings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE assets SET task_ids = array_remove(task_ids, $1) WHERE $1 = ANY(task_ids)`, taskID); err != nil {
		return fmt.Errorf("detach assets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM explorations WHERE id = $1`, t.ExplorationID); err != nil {
		return fmt.Errorf("delete exploration: %w", err)
	}

	return tx.Commit()
}

func (s *Store) EngagementActivity(ctx context.Context, explorationID int64, limit int) ([]tools.TimelineEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.QueryContext(ctx, `
		WITH task AS (
			SELECT id FROM tasks WHERE exploration_id = $1 LIMIT 1
		)
		SELECT ts, kind, summary FROM (
			SELECT created_at AS ts, 'finding' AS kind,
			       COALESCE(severity || ': ', '') || summary AS summary
			FROM findings WHERE task_id = (SELECT id FROM task)

			UNION ALL

			SELECT created_at AS ts, 'node:' || kind AS kind,
			       COALESCE(state, '') || ': ' || COALESCE(payload->>'text', payload->>'description', kind) AS summary
			FROM exploration_nodes WHERE exploration_id = $1

			UNION ALL

			SELECT created_at AS ts, 'asset' AS kind,
			       type || ': ' || COALESCE(domain, ip, url, app_name, service_name, '') AS summary
			FROM assets WHERE (SELECT id FROM task) = ANY(task_ids)
		) combined
		ORDER BY ts DESC
		LIMIT $2`, explorationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []tools.TimelineEntry
	for rows.Next() {
		var e tools.TimelineEntry
		var ts time.Time
		if err := rows.Scan(&ts, &e.Kind, &e.Summary); err != nil {
			return nil, err
		}
		e.Timestamp = ts.Format("15:04:05")
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEngagement(ctx context.Context, id int64) (*tools.EngagementDetail, error) {
	t, err := s.GetTask(id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	return &tools.EngagementDetail{
		EngagementSummary: tools.EngagementSummary{
			ID:            t.ID,
			Goal:          t.Goal,
			Status:        t.Status,
			ExplorationID: t.ExplorationID,
		},
		Description: t.Description,
	}, nil
}
