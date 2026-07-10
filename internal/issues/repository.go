package issues

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Issue is a local pi issue row loaded from .pi/issues.db.
//
// Newer pi schemas include Status, Thinking, ParentID, Owner, BlockedReason, and ClosedAt,
// and delegation records may include Model. Older databases may not have those columns;
// missing or NULL values are exposed as zero values or nil pointers.
type Issue struct {
	ID            int64
	Title         string
	Body          string
	State         string
	Status        string
	Thinking      string
	Model         string
	ParentID      *int64
	Owner         *string
	BlockedReason *string
	CreatedAt     string
	UpdatedAt     string
	ClosedAt      *string
}

// Repository loads issues from a read-only SQLite database.
type Repository struct {
	path              string
	db                *sql.DB
	columns           map[string]bool
	delegationColumns map[string]bool
}

// Open opens path as a read-only SQLite issue database and validates the issues table.
func Open(path string) (*Repository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("issues database path is empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve issues database path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("issues database %q does not exist", path)
		}
		return nil, fmt.Errorf("inspect issues database %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("issues database %q is a directory", path)
	}

	db, err := sql.Open("sqlite3", readOnlyDSN(absPath))
	if err != nil {
		return nil, fmt.Errorf("open issues database %q: %w", path, err)
	}

	repo := &Repository{path: path, db: db}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open issues database %q read-only: %w", path, err)
	}
	if err := repo.validate(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

func readOnlyDSN(absPath string) string {
	u := url.URL{Scheme: "file", Path: absPath}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_query_only", "1")
	u.RawQuery = q.Encode()
	return u.String()
}

func (r *Repository) validate() error {
	var name string
	err := r.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'issues'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("issues database %q does not contain an issues table", r.path)
	}
	if err != nil {
		return fmt.Errorf("inspect issues database %q: %w", r.path, err)
	}

	columns, err := loadColumns(r.db, "issues")
	if err != nil {
		return fmt.Errorf("inspect issues table in %q: %w", r.path, err)
	}

	var missing []string
	for _, column := range []string{"id", "title", "body", "state", "created_at", "updated_at"} {
		if !columns[column] {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("issues table in %q is missing required column(s): %s", r.path, strings.Join(missing, ", "))
	}

	delegationColumns, err := loadOptionalTableColumns(r.db, "issue_delegations")
	if err != nil {
		return fmt.Errorf("inspect issue_delegations table in %q: %w", r.path, err)
	}

	r.columns = columns
	r.delegationColumns = delegationColumns
	return nil
}

func loadOptionalTableColumns(db *sql.DB, table string) (map[string]bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return loadColumns(db, table)
}

func loadColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType sql.NullString
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

// Close closes the underlying database handle.
func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// List returns all issues ordered for browsing: open issues first, then status,
// most recently updated first, and finally by id for deterministic ties.
func (r *Repository) List(ctx context.Context) ([]Issue, error) {
	query := r.listQuery()
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load issues from %q: %w", r.path, err)
	}
	defer rows.Close()

	var result []Issue
	for rows.Next() {
		var (
			issue         Issue
			status        sql.NullString
			thinking      sql.NullString
			model         sql.NullString
			parentID      sql.NullInt64
			owner         sql.NullString
			blockedReason sql.NullString
			closedAt      sql.NullString
		)
		if err := rows.Scan(
			&issue.ID,
			&issue.Title,
			&issue.Body,
			&issue.State,
			&status,
			&thinking,
			&model,
			&parentID,
			&owner,
			&blockedReason,
			&issue.CreatedAt,
			&issue.UpdatedAt,
			&closedAt,
		); err != nil {
			return nil, fmt.Errorf("scan issue row from %q: %w", r.path, err)
		}
		if status.Valid {
			issue.Status = status.String
		}
		if thinking.Valid {
			issue.Thinking = thinking.String
		}
		if model.Valid {
			issue.Model = model.String
		}
		issue.ParentID = nullInt64Ptr(parentID)
		issue.Owner = nullStringPtr(owner)
		issue.BlockedReason = nullStringPtr(blockedReason)
		issue.ClosedAt = nullStringPtr(closedAt)
		result = append(result, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load issues from %q: %w", r.path, err)
	}
	return result, nil
}

func (r *Repository) listQuery() string {
	selects := []string{
		"issues.id",
		"issues.title",
		"issues.body",
		"issues.state",
		r.selectIssueColumnOrNull("status"),
		r.selectThinkingColumn(),
		r.selectDelegationModelColumn(),
		r.selectIssueColumnOrNull("parent_id"),
		r.selectIssueColumnOrNull("owner"),
		r.selectIssueColumnOrNull("blocked_reason"),
		"issues.created_at",
		"issues.updated_at",
		r.selectIssueColumnOrNull("closed_at"),
	}

	orderParts := []string{
		"CASE issues.state WHEN 'open' THEN 0 WHEN 'closed' THEN 1 ELSE 2 END",
	}
	if r.columns["status"] {
		orderParts = append(orderParts, "CASE issues.status WHEN 'in_progress' THEN 0 WHEN 'todo' THEN 1 WHEN 'blocked' THEN 2 WHEN 'done' THEN 3 ELSE 4 END")
	}
	orderParts = append(orderParts, "issues.updated_at DESC", "issues.id ASC")

	return fmt.Sprintf("SELECT %s FROM issues ORDER BY %s", strings.Join(selects, ", "), strings.Join(orderParts, ", "))
}

func (r *Repository) selectIssueColumnOrNull(name string) string {
	if r.columns[name] {
		return "issues." + name
	}
	return fmt.Sprintf("NULL AS %s", name)
}

func (r *Repository) selectThinkingColumn() string {
	var candidates []string
	if r.columns["thinking"] {
		candidates = append(candidates, "NULLIF(TRIM(issues.thinking), '')")
	}
	if r.columns["thinking_level"] {
		candidates = append(candidates, "NULLIF(TRIM(issues.thinking_level), '')")
	}
	if r.delegationColumns["issue_id"] && r.delegationColumns["thinking"] {
		orderParts := make([]string, 0, 2)
		if r.delegationColumns["updated_at"] {
			orderParts = append(orderParts, "issue_delegations.updated_at DESC")
		}
		if r.delegationColumns["id"] {
			orderParts = append(orderParts, "issue_delegations.id DESC")
		}
		orderClause := ""
		if len(orderParts) > 0 {
			orderClause = " ORDER BY " + strings.Join(orderParts, ", ")
		}
		candidates = append(candidates, fmt.Sprintf("(SELECT NULLIF(TRIM(issue_delegations.thinking), '') FROM issue_delegations WHERE issue_delegations.issue_id = issues.id AND issue_delegations.thinking IS NOT NULL AND TRIM(issue_delegations.thinking) != ''%s LIMIT 1)", orderClause))
	}
	if len(candidates) == 0 {
		return "NULL AS thinking"
	}
	if len(candidates) == 1 {
		return candidates[0] + " AS thinking"
	}
	return fmt.Sprintf("COALESCE(%s) AS thinking", strings.Join(candidates, ", "))
}

func (r *Repository) selectDelegationModelColumn() string {
	if !r.delegationColumns["issue_id"] || !r.delegationColumns["model"] {
		return "NULL AS model"
	}
	orderParts := make([]string, 0, 2)
	if r.delegationColumns["updated_at"] {
		orderParts = append(orderParts, "issue_delegations.updated_at DESC")
	}
	if r.delegationColumns["id"] {
		orderParts = append(orderParts, "issue_delegations.id DESC")
	}
	orderClause := ""
	if len(orderParts) > 0 {
		orderClause = " ORDER BY " + strings.Join(orderParts, ", ")
	}
	return fmt.Sprintf("(SELECT NULLIF(TRIM(issue_delegations.model), '') FROM issue_delegations WHERE issue_delegations.issue_id = issues.id AND issue_delegations.model IS NOT NULL AND TRIM(issue_delegations.model) != ''%s LIMIT 1) AS model", orderClause)
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

// OptionalColumns returns the optional issue columns present in this database.
func (r *Repository) OptionalColumns() []string {
	var present []string
	for _, column := range []string{"status", "thinking", "thinking_level", "parent_id", "owner", "blocked_reason", "closed_at"} {
		if r.columns[column] {
			present = append(present, column)
		}
	}
	if r.delegationColumns["thinking"] {
		present = append(present, "issue_delegations.thinking")
	}
	if r.delegationColumns["model"] {
		present = append(present, "issue_delegations.model")
	}
	sort.Strings(present)
	return present
}
