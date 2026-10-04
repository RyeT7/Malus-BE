package sqldb

import "testing"

func TestHasPassword(t *testing.T) {
	cases := map[string]bool{
		"sqlserver://localhost:1433?database=malus_content":                                      false,
		"sqlserver://malus.database.windows.net?database=content&fedauth=ActiveDirectoryDefault": false,
		"sqlserver://sa:secret@localhost:1433?database=malus_content":                            true,
		"sqlserver://localhost?database=x&password=secret":                                       true,
		"server=localhost;user id=sa;password=secret;database=x":                                 true,
		"server=localhost;database=x;Pwd=secret":                                                 true,
		"server=localhost;database=x;fedauth=ActiveDirectoryDefault":                             false,
	}
	for dsn, want := range cases {
		if got := hasPassword(dsn); got != want {
			t.Errorf("hasPassword(%q) = %v, want %v", dsn, got, want)
		}
	}
}

func TestOpenRejectsPasswordOutsideLocal(t *testing.T) {
	if _, err := Open("sqlserver://sa:secret@localhost?database=x", false); err == nil {
		t.Fatal("want error for password DSN when passwords are not allowed")
	}
	db, err := Open("sqlserver://sa:secret@localhost?database=x", true)
	if err != nil {
		t.Fatalf("local password DSN should be accepted: %v", err)
	}
	_ = db.Close()
}

func TestSplitBatches(t *testing.T) {
	body := "CREATE TABLE a (id INT);\nGO\n\n  go  \nCREATE TABLE b (id INT);\nGO\n"
	batches := splitBatches(body)
	if len(batches) != 2 || batches[0] != "CREATE TABLE a (id INT);" || batches[1] != "CREATE TABLE b (id INT);" {
		t.Fatalf("unexpected batches: %q", batches)
	}
}
