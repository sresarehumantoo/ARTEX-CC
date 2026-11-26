package main

import (
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestMatchesFindingTypeHyphenUnderscore(t *testing.T) {
	cases := []struct {
		name      string
		vulnClass string
		summary   string
		expected  string
		want      bool
	}{
		{"underscore expected, hyphen recorded", "privilege-escalation", "", "privilege_escalation", true},
		{"hyphen expected, underscore recorded", "privilege_escalation", "", "privilege-escalation", true},
		{"both hyphen", "privilege-escalation", "", "privilege-escalation", true},
		{"both underscore", "privilege_escalation", "", "privilege_escalation", true},
		{"mixed case recorded", "Privilege-Escalation", "", "privilege_escalation", true},
		{"mixed case expected", "privilege-escalation", "", "Privilege_Escalation", true},
		{"spaces recorded", "privilege escalation", "", "privilege_escalation", true},
		{"command injection hyphen vs underscore", "command-injection", "", "command_injection", true},
		{"directory traversal hyphen vs underscore", "directory-traversal", "", "directory_traversal", true},
		{"unrelated classes do not match", "sql-injection", "", "privilege_escalation", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tools.Finding{VulnClass: tc.vulnClass, Summary: tc.summary}
			if got := matchesFindingType(f, tc.expected); got != tc.want {
				t.Fatalf("matchesFindingType(%q, %q) = %v, want %v", tc.vulnClass, tc.expected, got, tc.want)
			}
		})
	}
}

func TestMatchesFindingTypeAliasesStillWork(t *testing.T) {
	cases := []struct {
		name      string
		vulnClass string
		summary   string
		expected  string
		want      bool
	}{
		{"sqli acronym vs sql_injection", "sqli", "", "sql_injection", true},
		{"sql injection summary", "", "Reflected SQL injection in login", "sql_injection", true},
		{"xss alias", "cross-site-scripting", "", "xss", true},
		{"idor broken access control", "idor", "", "broken_access_control", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tools.Finding{VulnClass: tc.vulnClass, Summary: tc.summary}
			if got := matchesFindingType(f, tc.expected); got != tc.want {
				t.Fatalf("matchesFindingType(vuln=%q, summary=%q, exp=%q) = %v, want %v", tc.vulnClass, tc.summary, tc.expected, got, tc.want)
			}
		})
	}
}
