package store

import (
	"testing"
)

func TestTokenize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []token
	}{
		{
			"empty",
			"",
			[]token{{tokenEOF, ""}},
		},
		{
			"single word",
			"example",
			[]token{{tokenWord, "example"}, {tokenEOF, ""}},
		},
		{
			"field=value",
			"domain=example.com",
			[]token{{tokenWord, "domain"}, {tokenOp, "="}, {tokenWord, "example.com"}, {tokenEOF, ""}},
		},
		{
			"field==value",
			"type==service",
			[]token{{tokenWord, "type"}, {tokenOp, "=="}, {tokenWord, "service"}, {tokenEOF, ""}},
		},
		{
			"field!=value",
			"type!=ip",
			[]token{{tokenWord, "type"}, {tokenOp, "!="}, {tokenWord, "ip"}, {tokenEOF, ""}},
		},
		{
			"greater/less operators",
			"port>80 port<443 port>=8080 port<=9090",
			[]token{
				{tokenWord, "port"}, {tokenOp, ">"}, {tokenWord, "80"},
				{tokenWord, "port"}, {tokenOp, "<"}, {tokenWord, "443"},
				{tokenWord, "port"}, {tokenOp, ">="}, {tokenWord, "8080"},
				{tokenWord, "port"}, {tokenOp, "<="}, {tokenWord, "9090"},
				{tokenEOF, ""},
			},
		},
		{
			"quoted value",
			`domain="example.com"`,
			[]token{{tokenWord, "domain"}, {tokenOp, "="}, {tokenQuoted, "example.com"}, {tokenEOF, ""}},
		},
		{
			"single-quoted value",
			`domain='example.com'`,
			[]token{{tokenWord, "domain"}, {tokenOp, "="}, {tokenQuoted, "example.com"}, {tokenEOF, ""}},
		},
		{
			"AND keyword",
			"a AND b",
			[]token{{tokenWord, "a"}, {tokenAnd, "AND"}, {tokenWord, "b"}, {tokenEOF, ""}},
		},
		{
			"OR keyword",
			"a OR b",
			[]token{{tokenWord, "a"}, {tokenOr, "OR"}, {tokenWord, "b"}, {tokenEOF, ""}},
		},
		{
			"|| operator",
			"a || b",
			[]token{{tokenWord, "a"}, {tokenOr, "||"}, {tokenWord, "b"}, {tokenEOF, ""}},
		},
		{
			"parentheses",
			"(a OR b)",
			[]token{{tokenLParen, "("}, {tokenWord, "a"}, {tokenOr, "OR"}, {tokenWord, "b"}, {tokenRParen, ")"}, {tokenEOF, ""}},
		},
		{
			"escaped quote",
			`"hello\"world"`,
			[]token{{tokenQuoted, `hello"world`}, {tokenEOF, ""}},
		},
		{
			"case insensitive keywords",
			"and or",
			[]token{{tokenAnd, "AND"}, {tokenOr, "OR"}, {tokenEOF, ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q) returned %d tokens, want %d\ngot:  %v\nwant: %v", tt.input, len(got), len(tt.want), got, tt.want)
			}
			for i := range tt.want {
				if got[i].kind != tt.want[i].kind || got[i].value != tt.want[i].value {
					t.Errorf("token[%d]: got {kind:%d, value:%q}, want {kind:%d, value:%q}",
						i, got[i].kind, got[i].value, tt.want[i].kind, tt.want[i].value)
				}
			}
		})
	}
}

func TestBuildDSLWhere(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		filter    string
		wantSQL   string
		wantArgs  int
		wantError bool
	}{
		{"empty", "", "TRUE", 0, false},
		{"whitespace only", "   ", "TRUE", 0, false},
		{
			"bare word search",
			"example",
			"(domain ILIKE $1 OR ip ILIKE $1 OR url ILIKE $1 OR page_title ILIKE $1)",
			1,
			false,
		},
		{
			"domain ilike",
			"domain=example",
			"domain ILIKE $1",
			1,
			false,
		},
		{
			"domain exact",
			"domain==example.com",
			"domain = $1",
			1,
			false,
		},
		{
			"domain not equal",
			"domain!=example.com",
			"(domain IS NULL OR domain != $1)",
			1,
			false,
		},
		{
			"type exact",
			"type=service",
			"type ILIKE $1",
			1,
			false,
		},
		{
			"port numeric",
			"port=80",
			"port = $1",
			1,
			false,
		},
		{
			"port greater than",
			"port>80",
			"port > $1",
			1,
			false,
		},
		{
			"port not numeric",
			"port=abc",
			"",
			0,
			true,
		},
		{
			"status_code numeric",
			"status_code=200",
			"status_code = $1",
			1,
			false,
		},
		{
			"technology ilike",
			"technology=nginx",
			"EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t ILIKE $1)",
			1,
			false,
		},
		{
			"technology exact",
			"technology==Nginx",
			"EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t = $2)",
			2,
			false,
		},
		{
			"technology not equal",
			"technology!=Nginx",
			"NOT EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t = $2)",
			2,
			false,
		},
		{
			"task_id",
			"task_id=42",
			"$1 = ANY(task_ids)",
			1,
			false,
		},
		{
			"task_id not equal",
			"task_id!=42",
			"NOT ($1 = ANY(task_ids))",
			1,
			false,
		},
		{
			"task_id non-numeric",
			"task_id=abc",
			"",
			0,
			true,
		},
		{
			"company_id",
			"company_id=1",
			"company_id = $1",
			1,
			false,
		},
		{
			"company_id not equal",
			"company_id!=1",
			"(company_id IS NULL OR company_id != $1)",
			1,
			false,
		},
		{
			"implicit AND",
			"domain=example type=service",
			"(domain ILIKE $1 AND type ILIKE $2)",
			2,
			false,
		},
		{
			"explicit AND",
			"domain=example AND type=service",
			"(domain ILIKE $1 AND type ILIKE $2)",
			2,
			false,
		},
		{
			"OR expression",
			"domain=foo OR domain=bar",
			"(domain ILIKE $1 OR domain ILIKE $2)",
			2,
			false,
		},
		{
			"|| expression",
			"domain=foo || domain=bar",
			"(domain ILIKE $1 OR domain ILIKE $2)",
			2,
			false,
		},
		{
			"parenthesized expression",
			"(domain=foo OR domain=bar) type=service",
			"((domain ILIKE $1 OR domain ILIKE $2) AND type ILIKE $3)",
			3,
			false,
		},
		{
			"unknown field becomes bare search",
			"custom=val",
			"(domain ILIKE $1 OR ip ILIKE $1 OR url ILIKE $1 OR page_title ILIKE $1)",
			1,
			false,
		},
		{
			"unclosed paren",
			"(domain=foo",
			"",
			0,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := buildDSLWhere(tt.filter)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got sql=%q args=%v", sql, args)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("sql:\n  got:  %s\n  want: %s", sql, tt.wantSQL)
			}
			if len(args) != tt.wantArgs {
				t.Errorf("args count: got %d, want %d (args=%v)", len(args), tt.wantArgs, args)
			}
		})
	}
}

func TestBuildDSLWhereTrailingInput(t *testing.T) {
	t.Parallel()
	_, _, err := buildDSLWhere("domain=foo )")
	if err == nil {
		t.Error("expected error for trailing input")
	}
}

func TestBuildDSLWhereExpectedValue(t *testing.T) {
	t.Parallel()
	_, _, err := buildDSLWhere("domain=")
	if err == nil {
		t.Error("expected error for missing value after operator")
	}
}

func TestBuildDSLWhereTechnologyUnsupportedOp(t *testing.T) {
	t.Parallel()
	_, _, err := buildDSLWhere("technology>foo")
	if err == nil {
		t.Error("expected error for unsupported operator on technology field")
	}
}

func TestBuildDSLWhereTextFieldUnsupportedOp(t *testing.T) {
	t.Parallel()
	_, _, err := buildDSLWhere("domain>foo")
	if err == nil {
		t.Error("expected error for > operator on text field")
	}
}
