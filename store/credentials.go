package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

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
	domain := DomainKey(cred.Domain)
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

// assetURLCredential extracts the userinfo NormalizeAssetURL strips from an
// asset URL, percent-decoded, with the host it belongs to.
func assetURLCredential(raw string) (domain, username, password string, ok bool) {
	stripped, userinfo, found := tools.SplitURLUserinfo(strings.TrimSpace(raw))
	if !found {
		return "", "", "", false
	}
	start, end := tools.URLAuthority(stripped)
	host := stripped[start:end]
	if strings.HasPrefix(host, "[") {
		host, _, _ = strings.Cut(host[1:], "]")
	} else {
		host, _, _ = strings.Cut(host, ":")
	}
	if strings.IndexFunc(host, unicode.IsSpace) >= 0 {
		return "", "", "", false
	}
	user, pass, _ := strings.Cut(userinfo, ":")
	domain = DomainKey(host)
	username = strings.TrimSpace(unescapeUserinfo(user))
	password = unescapeUserinfo(pass)
	return domain, username, password, domain != "" && username != ""
}

func unescapeUserinfo(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
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
