package config

import (
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	input := `
# comment
APP_ENV=local
export LOG_LEVEL=debug
SQL_DSN="sqlserver://localhost:1433?database=malus_content&encrypt=true"
QUOTED='a=b'
`
	vars, err := parseDotEnv(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"APP_ENV":   "local",
		"LOG_LEVEL": "debug",
		"SQL_DSN":   "sqlserver://localhost:1433?database=malus_content&encrypt=true",
		"QUOTED":    "a=b",
	}
	for k, v := range want {
		if vars[k] != v {
			t.Errorf("%s: want %q, got %q", k, v, vars[k])
		}
	}
}

func TestParseDotEnvRejectsMalformedLine(t *testing.T) {
	if _, err := parseDotEnv(strings.NewReader("NOT_A_PAIR")); err == nil {
		t.Fatal("want error for line without '='")
	}
}
