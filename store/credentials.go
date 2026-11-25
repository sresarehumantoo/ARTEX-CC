package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
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
	stored := password
	if s.credentialKey != nil && password != "" {
		enc, err := aesGCMEncrypt(s.credentialKey, password)
		if err != nil {
			return 0, fmt.Errorf("encrypt credential: %w", err)
		}
		stored = "enc:" + enc
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
		taskID, domain, username, stored, source, note).Scan(&id)
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
		if s.credentialKey != nil && strings.HasPrefix(c.Password, "enc:") {
			plain, err := aesGCMDecrypt(s.credentialKey, c.Password[4:])
			if err != nil {
				return nil, fmt.Errorf("decrypt credential %d: %w", c.ID, err)
			}
			c.Password = plain
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// assetURLCredential extracts the userinfo of an asset URL, percent-decoded.
// Only the authority is parsed, so a malformed path or query does not lose it.
func assetURLCredential(raw string) (domain, username, password string, ok bool) {
	raw = strings.TrimSpace(raw)
	i := strings.Index(raw, "://")
	if i < 0 {
		return "", "", "", false
	}
	if j := strings.IndexAny(raw[i+3:], "/?#"); j >= 0 {
		raw = raw[:i+3+j]
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return "", "", "", false
	}
	domain = DomainKey(u.Hostname())
	username = strings.TrimSpace(u.User.Username())
	password, _ = u.User.Password()
	return domain, username, password, domain != "" && username != ""
}

// saveAssetURLCredential records userinfo stripped from an asset URL. It never
// replaces an existing credential for the same domain and username.
func (s *Store) saveAssetURLCredential(ctx context.Context, tx *sql.Tx, taskID int64, rawURL string) error {
	domain, username, password, ok := assetURLCredential(rawURL)
	if !ok {
		return nil
	}
	if s.credentialKey != nil && password != "" {
		enc, err := aesGCMEncrypt(s.credentialKey, password)
		if err != nil {
			return fmt.Errorf("encrypt credential: %w", err)
		}
		password = "enc:" + enc
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO task_credentials (task_id, domain, username, password, source)
		VALUES ($1, $2, $3, $4, 'asset_url')
		ON CONFLICT (task_id, domain, username) DO NOTHING`,
		taskID, domain, username, password)
	return err
}
