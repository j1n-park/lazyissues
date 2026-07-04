package issues

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCurrentSchemaWithNullableOptionalFields(t *testing.T) {
	path := createTestDB(t, currentSchemaSQL, []string{
		`INSERT INTO issues (id, title, body, state, status, thinking, parent_id, owner, blocked_reason, created_at, updated_at, closed_at)
		 VALUES (1, 'closed issue', 'done body', 'closed', 'done', 'low', NULL, NULL, NULL, '2026-01-01T00:00:00Z', '2026-01-03T00:00:00Z', '2026-01-03T00:00:00Z')`,
		`INSERT INTO issues (id, title, body, state, status, thinking, parent_id, owner, blocked_reason, created_at, updated_at, closed_at)
		 VALUES (2, 'blocked issue', 'blocked body', 'open', 'blocked', 'high', 1, 'alice', 'waiting', '2026-01-01T00:00:00Z', '2026-01-04T00:00:00Z', NULL)`,
		`INSERT INTO issues (id, title, body, state, status, thinking, parent_id, owner, blocked_reason, created_at, updated_at, closed_at)
		 VALUES (3, 'active issue', 'active body', 'open', 'in_progress', 'medium', NULL, NULL, NULL, '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z', NULL)`,
	})

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	issues, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := len(issues), 3; got != want {
		t.Fatalf("len(List()) = %d, want %d", got, want)
	}

	// Open issues are ordered before closed issues, then by status priority.
	if got, want := issueIDs(issues), []int64{3, 2, 1}; !equalInt64s(got, want) {
		t.Fatalf("issue ids = %v, want %v", got, want)
	}

	active := issues[0]
	if active.Status != "in_progress" {
		t.Fatalf("active status = %q, want in_progress", active.Status)
	}
	if active.Thinking != "medium" {
		t.Fatalf("active thinking = %q, want medium", active.Thinking)
	}
	if active.ParentID != nil || active.Owner != nil || active.BlockedReason != nil || active.ClosedAt != nil {
		t.Fatalf("active nullable fields = %#v, want nil pointers", active)
	}

	blocked := issues[1]
	if blocked.ParentID == nil || *blocked.ParentID != 1 {
		t.Fatalf("blocked parent id = %v, want 1", blocked.ParentID)
	}
	if blocked.Owner == nil || *blocked.Owner != "alice" {
		t.Fatalf("blocked owner = %v, want alice", blocked.Owner)
	}
	if blocked.BlockedReason == nil || *blocked.BlockedReason != "waiting" {
		t.Fatalf("blocked reason = %v, want waiting", blocked.BlockedReason)
	}
	if blocked.Thinking != "high" {
		t.Fatalf("blocked thinking = %q, want high", blocked.Thinking)
	}
	if blocked.ClosedAt != nil {
		t.Fatalf("blocked closed_at = %v, want nil", blocked.ClosedAt)
	}

	closed := issues[2]
	if closed.ClosedAt == nil || *closed.ClosedAt != "2026-01-03T00:00:00Z" {
		t.Fatalf("closed_at = %v, want timestamp", closed.ClosedAt)
	}
}

func TestListOlderSchemaMissingOptionalColumns(t *testing.T) {
	path := createTestDB(t, olderSchemaSQL, []string{
		`INSERT INTO issues (id, title, body, state, created_at, updated_at)
		 VALUES (1, 'older closed', '', 'closed', '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z')`,
		`INSERT INTO issues (id, title, body, state, created_at, updated_at)
		 VALUES (2, 'older open', 'body', 'open', '2026-01-01T00:00:00Z', '2026-01-03T00:00:00Z')`,
	})

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	issues, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := issueIDs(issues), []int64{2, 1}; !equalInt64s(got, want) {
		t.Fatalf("issue ids = %v, want %v", got, want)
	}
	if got := issues[0].Status; got != "" {
		t.Fatalf("status for missing status column = %q, want empty string", got)
	}
	if got := issues[0].Thinking; got != "" {
		t.Fatalf("thinking for missing thinking source = %q, want empty string", got)
	}
	if issues[0].ParentID != nil || issues[0].Owner != nil || issues[0].BlockedReason != nil || issues[0].ClosedAt != nil {
		t.Fatalf("optional fields = %#v, want nil pointers", issues[0])
	}
	if got := repo.OptionalColumns(); len(got) != 0 {
		t.Fatalf("OptionalColumns() = %v, want none", got)
	}
}

func TestListUsesLatestDelegationThinkingWhenIssueColumnIsMissing(t *testing.T) {
	path := createTestDB(t, olderSchemaWithDelegationsSQL, []string{
		`INSERT INTO issues (id, title, body, state, created_at, updated_at)
		 VALUES (1, 'delegated issue', 'body', 'open', '2026-01-01T00:00:00Z', '2026-01-03T00:00:00Z')`,
		`INSERT INTO issues (id, title, body, state, created_at, updated_at)
		 VALUES (2, 'plain issue', 'body', 'open', '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z')`,
		`INSERT INTO issue_delegations (id, issue_id, scope, thinking, status, started_at, updated_at, output, stderr)
		 VALUES (1, 1, 'first scope', 'low', 'succeeded', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '', '')`,
		`INSERT INTO issue_delegations (id, issue_id, scope, thinking, status, started_at, updated_at, output, stderr)
		 VALUES (2, 1, 'latest scope', 'high', 'succeeded', '2026-01-01T00:00:00Z', '2026-01-04T00:00:00Z', '', '')`,
	})

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	issues, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := issues[0].Thinking, "high"; got != want {
		t.Fatalf("delegated issue thinking = %q, want %q", got, want)
	}
	if got := issues[1].Thinking; got != "" {
		t.Fatalf("plain issue thinking = %q, want empty string", got)
	}
	if got := repo.OptionalColumns(); !containsString(got, "issue_delegations.thinking") {
		t.Fatalf("OptionalColumns() = %v, want issue_delegations.thinking", got)
	}
}

func TestListSupportsThinkingLevelColumnAlias(t *testing.T) {
	path := createTestDB(t, thinkingLevelSchemaSQL, []string{
		`INSERT INTO issues (id, title, body, state, thinking_level, created_at, updated_at)
		 VALUES (1, 'aliased issue', 'body', 'open', 'low', '2026-01-01T00:00:00Z', '2026-01-03T00:00:00Z')`,
	})

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	issues, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := issues[0].Thinking, "low"; got != want {
		t.Fatalf("issue thinking = %q, want %q", got, want)
	}
}

func TestOpenMissingDatabaseErrorIsFriendly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open() error = nil, want missing database error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Open() error = %q, want user-friendly missing file message", err)
	}
}

func TestOpenMissingIssuesTableErrorIsFriendly(t *testing.T) {
	path := createTestDB(t, `CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)`, nil)
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open() error = nil, want missing issues table error")
	}
	if !strings.Contains(err.Error(), "does not contain an issues table") {
		t.Fatalf("Open() error = %q, want user-friendly missing table message", err)
	}
}

func TestOpenInvalidIssuesTableReportsMissingRequiredColumns(t *testing.T) {
	path := createTestDB(t, `CREATE TABLE issues (id INTEGER PRIMARY KEY, title TEXT)`, nil)
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open() error = nil, want invalid schema error")
	}
	if !strings.Contains(err.Error(), "missing required column") || !strings.Contains(err.Error(), "updated_at") {
		t.Fatalf("Open() error = %q, want missing required columns", err)
	}
}

func TestExampleIssuesDBLoads(t *testing.T) {
	path := filepath.Join("..", "..", "example_issues.db")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("example fixture %s is missing", path)
	}

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open(example_issues.db) error = %v", err)
	}
	defer repo.Close()

	issues, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List(example_issues.db) error = %v", err)
	}
	if len(issues) == 0 {
		t.Fatal("List(example_issues.db) returned no issues")
	}
	if issues[0].ID == 0 || issues[0].Title == "" || issues[0].CreatedAt == "" || issues[0].UpdatedAt == "" {
		t.Fatalf("first example issue missing required fields: %#v", issues[0])
	}
}

func createTestDB(t *testing.T, schema string, statements []string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "issues.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema error = %v", err)
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec statement %q error = %v", stmt, err)
		}
	}
	return path
}

func issueIDs(issues []Issue) []int64 {
	ids := make([]int64, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	return ids
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

const currentSchemaSQL = `
CREATE TABLE issues (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
	status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'blocked', 'done')),
	thinking TEXT CHECK (thinking IN ('low', 'medium', 'high')),
	parent_id INTEGER,
	owner TEXT,
	blocked_reason TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	closed_at TEXT
);`

const olderSchemaSQL = `
CREATE TABLE issues (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`

const olderSchemaWithDelegationsSQL = olderSchemaSQL + `
CREATE TABLE issue_delegations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
	role TEXT NOT NULL DEFAULT 'worker',
	scope TEXT NOT NULL,
	write_allowed INTEGER NOT NULL DEFAULT 0,
	thinking TEXT CHECK (thinking IN ('low', 'medium', 'high')),
	status TEXT NOT NULL CHECK (status IN ('preparing', 'running', 'succeeded', 'failed', 'canceled')),
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	output TEXT NOT NULL DEFAULT '',
	stderr TEXT NOT NULL DEFAULT ''
);`

const thinkingLevelSchemaSQL = `
CREATE TABLE issues (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
	thinking_level TEXT CHECK (thinking_level IN ('low', 'medium', 'high')),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`
