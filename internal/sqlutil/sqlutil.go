package sqlutil

import (
	"regexp"
	"strings"
)

// QuoteIdent wraps a SQL identifier in double quotes, escaping any embedded
// double quotes. Use for table names, view names, column names.
func QuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// QuoteLiteral wraps a SQL string literal in single quotes, escaping any
// embedded single quotes. Use for string values in SQL statements.
func QuoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

var simpleIdent = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// IsSimpleIdent returns true if s contains only letters, digits, and
// underscores, and does not start with a digit.
func IsSimpleIdent(s string) bool {
	return simpleIdent.MatchString(s)
}
