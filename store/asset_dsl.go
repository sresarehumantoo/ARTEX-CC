package store

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	tokenWord = iota
	tokenQuoted
	tokenOp
	tokenAnd
	tokenOr
	tokenLParen
	tokenRParen
	tokenEOF
)

type token struct {
	kind  int
	value string
}

func tokenize(input string) []token {
	var tokens []token
	runes := []rune(input)
	i := 0
	for i < len(runes) {
		ch := runes[i]
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		if ch == '(' {
			tokens = append(tokens, token{tokenLParen, "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{tokenRParen, ")"})
			i++
			continue
		}
		if ch == '|' && i+1 < len(runes) && runes[i+1] == '|' {
			tokens = append(tokens, token{tokenOr, "||"})
			i += 2
			continue
		}
		if ch == '!' && i+1 < len(runes) && runes[i+1] == '=' {
			tokens = append(tokens, token{tokenOp, "!="})
			i += 2
			continue
		}
		if ch == '=' && i+1 < len(runes) && runes[i+1] == '=' {
			tokens = append(tokens, token{tokenOp, "=="})
			i += 2
			continue
		}
		if ch == '=' {
			tokens = append(tokens, token{tokenOp, "="})
			i++
			continue
		}
		if ch == '>' && i+1 < len(runes) && runes[i+1] == '=' {
			tokens = append(tokens, token{tokenOp, ">="})
			i += 2
			continue
		}
		if ch == '>' {
			tokens = append(tokens, token{tokenOp, ">"})
			i++
			continue
		}
		if ch == '<' && i+1 < len(runes) && runes[i+1] == '=' {
			tokens = append(tokens, token{tokenOp, "<="})
			i += 2
			continue
		}
		if ch == '<' {
			tokens = append(tokens, token{tokenOp, "<"})
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote := ch
			i++
			var sb strings.Builder
			for i < len(runes) && runes[i] != quote {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
				}
				sb.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				i++ // closing quote
			}
			tokens = append(tokens, token{tokenQuoted, sb.String()})
			continue
		}
		var sb strings.Builder
		for i < len(runes) && !unicode.IsSpace(runes[i]) &&
			runes[i] != '(' && runes[i] != ')' &&
			runes[i] != '=' && runes[i] != '!' &&
			runes[i] != '>' && runes[i] != '<' &&
			runes[i] != '\'' && runes[i] != '"' &&
			!(runes[i] == '|' && i+1 < len(runes) && runes[i+1] == '|') {
			sb.WriteRune(runes[i])
			i++
		}
		word := sb.String()
		switch strings.ToUpper(word) {
		case "AND":
			tokens = append(tokens, token{tokenAnd, "AND"})
		case "OR":
			tokens = append(tokens, token{tokenOr, "OR"})
		default:
			tokens = append(tokens, token{tokenWord, word})
		}
	}
	tokens = append(tokens, token{tokenEOF, ""})
	return tokens
}

var fieldToColumn = map[string]string{
	"domain":       "domain",
	"root_domain":  "root_domain",
	"ip":           "ip",
	"url":          "url",
	"page_title":   "page_title",
	"service_name": "service_name",
	"port":         "port",
	"status_code":  "status_code",
	"technology":   "technology",
	"task_id":      "task_id",
	"company_id":   "company_id",
	"type":         "type",
}

var numericFields = map[string]bool{
	"port":        true,
	"status_code": true,
}

type dslParser struct {
	tokens    []token
	pos       int
	args      []any
	nextParam int
}

func (p *dslParser) peek() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{tokenEOF, ""}
}

func (p *dslParser) advance() token {
	t := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

func (p *dslParser) addParam(v any) string {
	p.nextParam++
	p.args = append(p.args, v)
	return fmt.Sprintf("$%d", p.nextParam)
}

func (p *dslParser) parseExpr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.peek().kind == tokenOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return "", err
		}
		left = "(" + left + " OR " + right + ")"
	}
	return left, nil
}

func (p *dslParser) parseAnd() (string, error) {
	left, err := p.parseAtom()
	if err != nil {
		return "", err
	}
	for {
		t := p.peek()
		if t.kind == tokenAnd {
			p.advance()
			right, err := p.parseAtom()
			if err != nil {
				return "", err
			}
			left = "(" + left + " AND " + right + ")"
			continue
		}
		if t.kind == tokenWord || t.kind == tokenQuoted || t.kind == tokenLParen {
			right, err := p.parseAtom()
			if err != nil {
				return "", err
			}
			left = "(" + left + " AND " + right + ")"
			continue
		}
		break
	}
	return left, nil
}

func (p *dslParser) parseAtom() (string, error) {
	t := p.peek()
	if t.kind == tokenLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return "", err
		}
		if p.peek().kind != tokenRParen {
			return "", fmt.Errorf("expected closing parenthesis")
		}
		p.advance()
		return expr, nil
	}
	if t.kind == tokenWord || t.kind == tokenQuoted {
		p.advance()
		op := p.peek()
		if op.kind == tokenOp {
			return p.parseComparison(t.value, op.value)
		}
		return p.bareTextSearch(t.value), nil
	}
	return "", fmt.Errorf("unexpected token: %q", t.value)
}

func (p *dslParser) parseComparison(field, op string) (string, error) {
	p.advance() // consume operator
	val := p.peek()
	if val.kind != tokenWord && val.kind != tokenQuoted {
		return "", fmt.Errorf("expected value after %s%s", field, op)
	}
	p.advance()
	value := val.value

	col, known := fieldToColumn[strings.ToLower(field)]
	if !known {
		return p.bareFieldSearch(field, op, value), nil
	}
	fieldLower := strings.ToLower(field)

	if fieldLower == "url" && (op == "==" || op == "!=") {
		value = NormalizeAssetURL(value)
	}

	if fieldLower == "technology" {
		param := p.addParam("%" + value + "%")
		switch op {
		case "=":
			return "EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t ILIKE " + param + ")", nil
		case "==":
			param = p.addParam(value)
			return "EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t = " + param + ")", nil
		case "!=":
			param = p.addParam(value)
			return "NOT EXISTS (SELECT 1 FROM unnest(technologies) t WHERE t = " + param + ")", nil
		default:
			return "", fmt.Errorf("operator %s not supported for technology", op)
		}
	}

	if fieldLower == "task_id" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("task_id requires an integer value")
		}
		param := p.addParam(n)
		switch op {
		case "=", "==":
			return param + " = ANY(task_ids)", nil
		case "!=":
			return "NOT (" + param + " = ANY(task_ids))", nil
		default:
			return "", fmt.Errorf("operator %s not supported for task_id", op)
		}
	}

	if numericFields[fieldLower] {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("%s requires a numeric value", field)
		}
		param := p.addParam(n)
		sqlOp := op
		if sqlOp == "==" {
			sqlOp = "="
		}
		if sqlOp == "=" {
			return col + " = " + param, nil
		}
		return col + " " + sqlOp + " " + param, nil
	}

	if fieldLower == "company_id" {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("company_id requires an integer value")
		}
		param := p.addParam(n)
		switch op {
		case "=", "==":
			return col + " = " + param, nil
		case "!=":
			return "(" + col + " IS NULL OR " + col + " != " + param + ")", nil
		default:
			return "", fmt.Errorf("operator %s not supported for company_id", op)
		}
	}

	switch op {
	case "=":
		param := p.addParam("%" + value + "%")
		return col + " ILIKE " + param, nil
	case "==":
		param := p.addParam(value)
		return col + " = " + param, nil
	case "!=":
		param := p.addParam(value)
		return "(" + col + " IS NULL OR " + col + " != " + param + ")", nil
	default:
		return "", fmt.Errorf("operator %s not supported for text field %s", op, field)
	}
}

func (p *dslParser) bareTextSearch(text string) string {
	param := p.addParam("%" + text + "%")
	return "(domain ILIKE " + param +
		" OR ip ILIKE " + param +
		" OR url ILIKE " + param +
		" OR page_title ILIKE " + param + ")"
}

func (p *dslParser) bareFieldSearch(field, op, value string) string {
	combined := field + op + value
	return p.bareTextSearch(combined)
}

func buildDSLWhere(filter string) (string, []any, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return "TRUE", nil, nil
	}
	tokens := tokenize(filter)
	parser := &dslParser{tokens: tokens, nextParam: 0}
	clause, err := parser.parseExpr()
	if err != nil {
		return "", nil, fmt.Errorf("parse filter: %w", err)
	}
	if parser.peek().kind != tokenEOF {
		return "", nil, fmt.Errorf("unexpected trailing input: %q", parser.peek().value)
	}
	return clause, parser.args, nil
}
