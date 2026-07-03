# lazyissues

`lazyissues` is a first-iteration, read-only terminal UI for browsing local pi issue queues stored in SQLite.
It provides a lazygit-inspired split view with an issue list, issue detail pane, keyboard navigation, and non-interactive fallback rendering for scripts or smoke checks.

## Status and limitations

- Read-only: `lazyissues` never edits, closes, reopens, assigns, or otherwise mutates issues.
- SQLite-backed: it reads the `issues` table from a local pi issue database.
- First iteration: screenshots/asciinema demos are not included yet; use the example database smoke command below to preview the layout.

## Requirements

- Go 1.22 or newer.
- CGO-capable Go toolchain for `github.com/mattn/go-sqlite3`.
  - Linux typically needs a C compiler such as `gcc` installed.
  - If CGO is disabled, builds that include sqlite support will fail.

## Install

Install from this checkout into `~/.local/bin`:

```sh
./install.sh
```

Use `PREFIX` or `BINDIR` to choose a different install location:

```sh
PREFIX=/usr/local ./install.sh
BINDIR=/tmp/bin ./install.sh
```

Uninstall the binary from the same location:

```sh
./uninstall.sh
PREFIX=/usr/local ./uninstall.sh
BINDIR=/tmp/bin ./uninstall.sh
```

You can also install with Go directly into your `GOBIN`/`GOPATH/bin`:

```sh
go install ./cmd/lazyissues
```

Or build a local binary:

```sh
go build -o lazyissues ./cmd/lazyissues
```

## Run

Open the default local pi issue database:

```sh
go run ./cmd/lazyissues
# or, after building/installing:
./lazyissues
```

Open a specific database:

```sh
go run ./cmd/lazyissues --db ./example_issues.db
./lazyissues --db /path/to/.pi/issues.db
```

Print version/help information:

```sh
go run ./cmd/lazyissues --version
go run ./cmd/lazyissues --help
```

When stdout is not an interactive terminal, `lazyissues` prints a fixed-size TUI snapshot and exits. This is useful for CI or quick smoke checks:

```sh
go run ./cmd/lazyissues --db ./example_issues.db | head -40
```

## Database discovery

By default, `lazyissues` opens:

```text
./.pi/issues.db
```

Run it from the root of a project that uses the local pi issue queue, or pass `--db` with an explicit path. The app validates that the file exists, is a SQLite database, and contains an `issues` table with the required columns (`id`, `title`, `body`, `state`, `created_at`, `updated_at`). Optional columns such as `status`, `parent_id`, `owner`, `blocked_reason`, and `closed_at` are displayed when present.

## Keybindings

- `q` / `Esc` / `Ctrl+C`: quit.
- `j` / `Down`: move down in the list or scroll down in the detail pane.
- `k` / `Up`: move up in the list or scroll up in the detail pane.
- `Page Up` / `Page Down`: jump by a page in the focused pane.
- `Home` / `End`: jump to the first/last item or detail line.
- `Tab`: toggle focus between the issue list and detail panes.
- `h` / `Left`: focus the issue list.
- `l` / `Right`: focus the detail pane.
- `?`: show or hide the expanded help footer.
- `r`: refresh/reload issues from the database without reopening the TUI.

When the detail pane is focused and the selected issue body contains Markdown-like headings:

- `Enter` / `Space`: toggle the section at or above the current detail scroll position.
- `a`: expand all sections in the selected issue body.
- `z`: collapse all sections in the selected issue body.
- `[` / `]`: jump to the previous/next section heading.

The footer always shows the current focus and a compact keybinding reminder.

## Issue body rendering

Issue bodies are stored and read as plain text. They are commonly written in a Markdown style, but `lazyissues` does not implement full Markdown rendering.

For readability, the detail pane recognizes Markdown-like ATX headings (`# Heading` through `###### Heading`) as issue body sections. Recognized headings are rendered as colored section headers with disclosure markers (`▾` expanded, `▸` collapsed). Collapsing a section hides its text and lower-level subsections until the next heading at the same or higher level.

Collapse state is read-only UI state held in memory for the current TUI session. Toggling sections never changes the issue body or writes anything back to the SQLite database.

## Rendering states

The UI renders:

- Loading/startup state while the terminal size is not known yet.
- Error state for missing databases, invalid schemas, and load failures.
- Empty state when the database loads but contains no issues.
- Open and closed issue states.
- Common statuses including `todo`, `in_progress`, `blocked`, and `done`.
- Optional parent, owner, blocked reason, and closed timestamp metadata when available.

## Development and validation

Format, test, build, and smoke-check the first iteration:

```sh
gofmt -w ./cmd/lazyissues ./internal/issues ./internal/tui
go test ./...
go build ./cmd/lazyissues
go run ./cmd/lazyissues --db ./example_issues.db | head -40
```
