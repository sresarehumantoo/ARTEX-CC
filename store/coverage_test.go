package store

import (
	"testing"
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
