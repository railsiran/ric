package commands

import (
	"strings"
	"testing"
)

const plainDBYaml = `default: &default
  adapter: sqlite3
  timeout: 5000

development:
  <<: *default
  database: storage/development.sqlite3

production:
  primary:
    <<: *default
    database: storage/production.sqlite3
`

func TestAddSQLitePragmasInsertsWAL(t *testing.T) {
	out, changed := addSQLitePragmas(plainDBYaml)
	if !changed {
		t.Fatal("expected pragmas to be inserted")
	}
	for _, want := range []string{
		"pragmas:",
		"journal_mode: WAL",
		"synchronous: NORMAL",
		"busy_timeout: 5000",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	// The pragmas: key should sit at the same indent as other default-block
	// keys (2 spaces here), nested children one level deeper (4 spaces).
	if !strings.Contains(out, "\n  pragmas:\n    journal_mode: WAL") {
		t.Errorf("pragmas block not indented as expected:\n%s", out)
	}
}

func TestAddSQLitePragmasIdempotent(t *testing.T) {
	once, _ := addSQLitePragmas(plainDBYaml)
	twice, changed := addSQLitePragmas(once)
	if changed {
		t.Error("second call should be a no-op")
	}
	if once != twice {
		t.Error("idempotent call changed the content")
	}
}

func TestAddSQLitePragmasSkipsNonSQLite(t *testing.T) {
	pg := "default: &default\n  adapter: postgresql\n  host: localhost\n"
	if _, changed := addSQLitePragmas(pg); changed {
		t.Error("must not touch non-sqlite configs")
	}
}

func TestAddSQLitePragmasSkipsWhenNoAnchor(t *testing.T) {
	// adapter present but no `default:` anchor to attach to.
	noAnchor := "production:\n  adapter: sqlite3\n  database: storage/production.sqlite3\n"
	if _, changed := addSQLitePragmas(noAnchor); changed {
		t.Error("must not act without a default anchor")
	}
}

func TestAddSQLitePragmasRespectsExisting(t *testing.T) {
	existing := plainDBYaml + "\n  pragmas:\n    journal_mode: WAL\n"
	if _, changed := addSQLitePragmas(existing); changed {
		t.Error("must not double-insert when pragmas already present")
	}
}
