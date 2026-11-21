package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

const (
	GapNeedsFollowUp       = "needs_follow_up"
	GapUnrecordedRiskClass = "unrecorded_risk_class"
	GapMissingEvidence     = "missing_evidence"
	GapMissingReason       = "missing_reason"
)

const (
	maxCoverageSurface     = 200
	maxCoverageRiskArea    = 64
	maxCoverageEvidence    = 500
	maxCoverageReason      = 500
	maxCoverageRowsPerTask = 2000
)

type CoverageEntry struct {
	Surface   string    `json:"surface"`
	RiskArea  string    `json:"risk_area"`
	Outcome   string    `json:"outcome"`
	Evidence  string    `json:"evidence"`
	Reason    string    `json:"reason"`
	Worker    string    `json:"worker,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CoverageStats struct {
	Entries    []CoverageEntry `json:"entries"`
	Gaps       []CoverageGap   `json:"gaps"`
	Unexamined []string        `json:"unexamined"`
}

type CoverageGap struct {
	Kind     string `json:"kind"`
	Surface  string `json:"surface,omitempty"`
	RiskArea string `json:"risk_area"`
}

func ValidCoverageOutcome(o string) bool {
	switch o {
	case OutcomeReported, OutcomeNoIssueFound, OutcomeRuledOut, OutcomeNotApplicable, OutcomeNeedsFollowUp:
		return true
	}
	return false
}

func normalizeCoverageField(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func normRiskArea(s string) string { return strings.ToLower(normalizeCoverageField(s)) }


func (s *Store) RecordCoverage(ctx context.Context, taskID int64, entry tools.CoverageEntry) error {
	if taskID <= 0 {
		return fmt.Errorf("task_id is required")
	}
	e := CoverageEntry{
		Surface:  normalizeCoverageField(entry.Surface),
		RiskArea: normRiskArea(entry.RiskArea),
		Outcome:  entry.Outcome,
		Evidence: normalizeCoverageField(entry.Evidence),
		Reason:   normalizeCoverageField(entry.Reason),
		Worker:   entry.Worker,
	}
	if e.Surface == "" || e.RiskArea == "" {
		return fmt.Errorf("surface and risk_area are required")
	}
	if !ValidCoverageOutcome(e.Outcome) {
		return fmt.Errorf("outcome must be one of reported, no_issue_found, ruled_out, not_applicable, needs_follow_up")
	}
	for _, f := range []struct {
		name string
		v    string
		max  int
	}{
		{"surface", e.Surface, maxCoverageSurface},
		{"risk_area", e.RiskArea, maxCoverageRiskArea},
		{"evidence", e.Evidence, maxCoverageEvidence},
		{"reason", e.Reason, maxCoverageReason},
	} {
		if utf8.RuneCountInString(f.v) > f.max {
			return fmt.Errorf("%s too long (max %d)", f.name, f.max)
		}
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM tasks WHERE id=$1 FOR UPDATE`, taskID); err != nil {
		return err
	}

	var exists bool
	var current int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM coverage_ledger WHERE task_id=$1 AND surface=$2 AND risk_area=$3),
		(SELECT COUNT(*) FROM coverage_ledger WHERE task_id=$1)`, taskID, e.Surface, e.RiskArea).Scan(&exists, &current); err != nil {
		return err
	}
	if !exists && current >= maxCoverageRowsPerTask {
		return fmt.Errorf("coverage ledger full for this task (max %d entries)", maxCoverageRowsPerTask)
	}

	args := []any{taskID, e.Surface, e.RiskArea, e.Outcome, e.Evidence, e.Reason, e.Worker}
	if _, err := tx.ExecContext(ctx, `INSERT INTO coverage_ledger_history(task_id, surface, risk_area, outcome, evidence, reason, worker)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, args...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO coverage_ledger(task_id, surface, risk_area, outcome, evidence, reason, worker)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (task_id, surface, risk_area) DO UPDATE SET outcome=EXCLUDED.outcome, evidence=EXCLUDED.evidence,
		reason=EXCLUDED.reason, worker=EXCLUDED.worker, updated_at=now()`, args...); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) CoverageStats(taskID int64) (*CoverageStats, error) {
	rows, err := s.Query(`SELECT surface, risk_area, outcome, evidence, reason, worker, updated_at
		FROM coverage_ledger WHERE task_id=$1 ORDER BY risk_area, surface`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []CoverageEntry
	for rows.Next() {
		var e CoverageEntry
		if err := rows.Scan(&e.Surface, &e.RiskArea, &e.Outcome, &e.Evidence, &e.Reason, &e.Worker, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var classes []string
	crows, err := s.Query(`SELECT DISTINCT lower(btrim(n.payload->>'vulnclass'))
		FROM exploration_nodes n
		JOIN tasks t ON t.exploration_id = n.exploration_id
		WHERE t.id=$1 AND n.kind='goal' AND COALESCE(btrim(n.payload->>'vulnclass'),'') <> ''
		ORDER BY 1`, taskID)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var c string
		if err := crows.Scan(&c); err != nil {
			return nil, err
		}
		classes = append(classes, c)
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}

	stats := &CoverageStats{
		Entries:    entries,
		Gaps:       []CoverageGap{},
		Unexamined: []string{},
	}
	if stats.Entries == nil {
		stats.Entries = []CoverageEntry{}
	}

	seen := map[string]bool{}
	for _, e := range entries {
		seen[normRiskArea(e.RiskArea)] = true
		switch {
		case e.Outcome == OutcomeNeedsFollowUp:
			stats.Gaps = append(stats.Gaps, CoverageGap{GapNeedsFollowUp, e.Surface, e.RiskArea})
		case e.Outcome == OutcomeNoIssueFound && strings.TrimSpace(e.Evidence) == "":
			stats.Gaps = append(stats.Gaps, CoverageGap{GapMissingEvidence, e.Surface, e.RiskArea})
		case (e.Outcome == OutcomeRuledOut || e.Outcome == OutcomeNotApplicable) && strings.TrimSpace(e.Reason) == "":
			stats.Gaps = append(stats.Gaps, CoverageGap{GapMissingReason, e.Surface, e.RiskArea})
		}
	}

	deduped := map[string]bool{}
	for _, m := range classes {
		m = normRiskArea(m)
		if m != "" && !seen[m] && !deduped[m] {
			deduped[m] = true
			stats.Unexamined = append(stats.Unexamined, m)
		}
	}
	sort.Strings(stats.Unexamined)

	unrecorded := make([]CoverageGap, 0, len(stats.Unexamined))
	for _, m := range stats.Unexamined {
		unrecorded = append(unrecorded, CoverageGap{Kind: GapUnrecordedRiskClass, RiskArea: m})
	}
	stats.Gaps = append(unrecorded, stats.Gaps...)

	return stats, nil
}
