package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/azuread"
)

func Open(dsn string, allowPassword bool) (*sql.DB, error) {
	if !allowPassword && hasPassword(dsn) {
		return nil, errors.New("SQL_DSN contains a password, which is only allowed when APP_ENV=local; use fedauth=ActiveDirectoryDefault with a managed identity")
	}
	connector, err := azuread.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse SQL_DSN: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func IsUniqueViolation(err error) bool {
	var e mssql.Error
	return errors.As(err, &e) && (e.Number == 2627 || e.Number == 2601)
}

func hasPassword(dsn string) bool {
	if u, err := url.Parse(dsn); err == nil && strings.EqualFold(u.Scheme, "sqlserver") {
		if _, ok := u.User.Password(); ok {
			return true
		}
		for key := range u.Query() {
			if strings.EqualFold(key, "password") {
				return true
			}
		}
		return false
	}
	for _, part := range strings.Split(dsn, ";") {
		key, _, _ := strings.Cut(part, "=")
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "password", "pwd":
			return true
		}
	}
	return false
}
