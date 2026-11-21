package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

type Credential struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	Domain    string    `json:"domain"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Source    string    `json:"source"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var errCredentialInvalid = errors.New("credential domain and username are required")

func (s *Store) SaveCredential(ctx context.Context, taskID int64, cred tools.CredentialInput) (int64, error) {
	domain := strings.TrimSpace(cred.Domain)
	username := strings.TrimSpace(cred.Username)
	password := strings.TrimSpace(cred.Password)
	note := strings.TrimSpace(cred.Note)
	source := cred.Source
	if source == "" {
		source = "agent"
	}
	if domain == "" || username == "" {
		return 0, errCredentialInvalid
	}
	if password == "" && source == "agent" {
		return 0, fmt.Errorf("agent-recorded credentials require a password")
	}
	var id int64
	err := s.QueryRowContext(ctx, `
		INSERT INTO task_credentials (task_id, domain, username, password, source, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (task_id, domain, username) DO UPDATE
		SET password   = EXCLUDED.password,
		    source     = EXCLUDED.source,
		    note       = CASE WHEN EXCLUDED.note = '' THEN task_credentials.note ELSE EXCLUDED.note END,
		    updated_at = now()
		RETURNING id`,
		taskID, domain, username, password, source, note).Scan(&id)
	return id, err
}

func (s *Store) ListCredentials(ctx context.Context, taskID int64) ([]tools.Credential, error) {
	rows, err := s.QueryContext(ctx, `
		SELECT id, domain, username, password, source, note
		FROM task_credentials WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tools.Credential
	for rows.Next() {
		var c tools.Credential
		if err := rows.Scan(&c.ID, &c.Domain, &c.Username, &c.Password, &c.Source, &c.Note); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
