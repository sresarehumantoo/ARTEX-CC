package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

type Task struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Goal          string `json:"goal"`
	ExplorationID int64  `json:"exploration_id"`
	Status        string `json:"status"`
	CompanyID     *int64 `json:"company_id,omitempty"`
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
		SELECT id, goal, COALESCE(status,''), exploration_id
		FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.EngagementSummary
	for rows.Next() {
		var e tools.EngagementSummary
		if err := rows.Scan(&e.ID, &e.Goal, &e.Status, &e.ExplorationID); err != nil {
			return nil, err
		}
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
