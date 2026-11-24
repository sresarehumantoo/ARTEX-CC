package store

import (
	"context"
	"strings"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestValidCoverageOutcome(t *testing.T) {
	t.Parallel()
	valid := []string{
		OutcomeReported,
		OutcomeNoIssueFound,
		OutcomeRuledOut,
		OutcomeNotApplicable,
		OutcomeNeedsFollowUp,
	}
	for _, o := range valid {
		if !ValidCoverageOutcome(o) {
			t.Errorf("ValidCoverageOutcome(%q) = false, want true", o)
		}
	}

	invalid := []string{"", "invalid", "REPORTED", "Reported", "pending"}
	for _, o := range invalid {
		if ValidCoverageOutcome(o) {
			t.Errorf("ValidCoverageOutcome(%q) = true, want false", o)
		}
	}
}

func TestNormalizeCoverageField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"  hello   world  ", "hello world"},
		{"tab\there", "tab here"},
		{"normal", "normal"},
		{"", ""},
		{"multi\n\nline", "multi line"},
	}
	for _, tt := range tests {
		got := normalizeCoverageField(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCoverageField(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRecordCoverageValidation(t *testing.T) {
	t.Parallel()
	s := &Store{}
	ctx := context.Background()

	t.Run("zero task_id", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 0, tools.CoverageEntry{
			Surface: "web", RiskArea: "xss", Outcome: OutcomeReported,
		})
		if err == nil || !strings.Contains(err.Error(), "task_id") {
			t.Errorf("want task_id error, got %v", err)
		}
	})

	t.Run("empty surface", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			RiskArea: "xss", Outcome: OutcomeReported,
		})
		if err == nil || !strings.Contains(err.Error(), "surface") {
			t.Errorf("want surface error, got %v", err)
		}
	})

	t.Run("empty risk_area", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface: "web", Outcome: OutcomeReported,
		})
		if err == nil || !strings.Contains(err.Error(), "surface") {
			t.Errorf("want risk_area error, got %v", err)
		}
	})

	t.Run("invalid outcome", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface: "web", RiskArea: "xss", Outcome: "bogus",
		})
		if err == nil || !strings.Contains(err.Error(), "outcome") {
			t.Errorf("want outcome error, got %v", err)
		}
	})

	t.Run("surface too long", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface:  strings.Repeat("x", 201),
			RiskArea: "xss",
			Outcome:  OutcomeReported,
		})
		if err == nil || !strings.Contains(err.Error(), "surface too long") {
			t.Errorf("want surface too long error, got %v", err)
		}
	})

	t.Run("risk_area too long", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface:  "web",
			RiskArea: strings.Repeat("x", 65),
			Outcome:  OutcomeReported,
		})
		if err == nil || !strings.Contains(err.Error(), "risk_area too long") {
			t.Errorf("want risk_area too long error, got %v", err)
		}
	})

	t.Run("evidence too long", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface:  "web",
			RiskArea: "xss",
			Outcome:  OutcomeReported,
			Evidence: strings.Repeat("x", 501),
		})
		if err == nil || !strings.Contains(err.Error(), "evidence too long") {
			t.Errorf("want evidence too long error, got %v", err)
		}
	})

	t.Run("reason too long", func(t *testing.T) {
		err := s.RecordCoverage(ctx, 1, tools.CoverageEntry{
			Surface:  "web",
			RiskArea: "xss",
			Outcome:  OutcomeRuledOut,
			Reason:   strings.Repeat("x", 501),
		})
		if err == nil || !strings.Contains(err.Error(), "reason too long") {
			t.Errorf("want reason too long error, got %v", err)
		}
	})
}

func TestNormRiskArea(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"Authentication", "authentication"},
		{"  SQL  Injection  ", "sql injection"},
		{"XSS", "xss"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normRiskArea(tt.input)
		if got != tt.want {
			t.Errorf("normRiskArea(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
