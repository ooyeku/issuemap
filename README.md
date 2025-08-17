# IssueMap

Git‑native issue tracking you can version, review, and ship alongside your code. IssueMap stores issues as YAML files in your repository, gives you a fast CLI, and includes a simple web UI backed by a lightweight HTTP API.

- Keep issues in git under `.issuemap/` so they travel with branches and PRs
- Work fast from the terminal (create, list, edit, link, bulk, time tracking)
- Start a local API server and open the bundled web UI when you need a visual view


## Quick start

Prerequisites:
- Git
- Go 1.24+ (module path: `github.com/ooyeku/issuemap`)

Install:
```bash
# using Go
go install github.com/ooyeku/issuemap@latest

# or via Makefile (adds version info)
make install              # installs to $(go env GOPATH)/bin
issuemap --version
```

Initialize in a repo:
```bash
cd /path/to/your/repo
issuemap init
```

Create and list issues:
```bash
issuemap create --title "Fix flaky CI" --type bug --priority high --label ci
issuemap list
```

Open the web UI (starts server if needed):
```bash
issuemap web
# opens http://localhost:<port>/ (default port 4042; see below)
```

Run the server directly:
```bash
# start / stop / status
issuemap server start
issuemap server status
issuemap server stop

# defaults
# - base path:   .issuemap/
# - API base:    /api/v1
# - default port: 4042 (auto-picks 4042–4052 if occupied)
```

A common workflow:
```bash
# create → branch → work → sync → close
issuemap create --title "Implement search" --type feature
issuemap branch ISSUE-123   # creates/links a branch
# ... commit as usual, reference the issue ID in messages if you like ...
issuemap sync               # picks up changes, updates metadata/history
issuemap close ISSUE-123 --reason done
```


## How it’s laid out on disk
IssueMap keeps everything under `.issuemap/` in your repo:
- `.issuemap/issues/` — issue YAML files
- `.issuemap/history/` — change history
- `.issuemap/templates/` — optional templates
- `.issuemap/metadata/` — server/runtime metadata (logs, etc.)

These files are plain text and reviewable in PRs. Its recommended projects commit the issue/history/templates directories and ignore a few runtime files (see .gitignore below).


## CLI highlights
A sampling of useful commands:

### Core Issue Management
- `issuemap init` — set up `.issuemap/` in the current git repo
- `issuemap create` — create a new issue (flags for title, type, priority, labels)
- `issuemap list` — list issues with filters and sorting
- `issuemap show ISSUE-123` — show details for one issue
- `issuemap edit ISSUE-123` — edit fields
- `issuemap assign` — assign/unassign issues to team members
- `issuemap close` / `delete` — close or remove issues

### Advanced Features
- `issuemap search` — advanced search with query syntax and saved searches
- `issuemap depend` / `deps` — manage dependencies between issues
- `issuemap template` — create and manage issue templates
- `issuemap bulk` — bulk operations (labels, status, assignments, etc.)
- `issuemap attach` — attach files to issues with compression support
- `issuemap dedup` — find and merge duplicate issues

### Time Tracking & Reports
- `issuemap estimate` — estimate work time for issues
- `issuemap start` / `stop` — time tracking timers
- `issuemap log` — manual time logging
- `issuemap report` — generate time/velocity/burndown reports
- `issuemap velocity` / `burndown` — team performance analytics

### Git Integration
- `issuemap branch` — create Git branches linked to issues
- `issuemap sync` — sync changes with Git repository

### Data Management
- `issuemap archive` — archive old issues with compression
- `issuemap restore` — restore from backups
- `issuemap storage` — manage storage and optimize performance
- `issuemap cleanup` — clean temporary files and optimize storage
- `issuemap compress` — manage attachment compression settings

### Server & Web UI
- `issuemap server ...` — start/stop/status for the local API
- `issuemap web` — open the web UI
- `issuemap logs` — view system logs

### Productivity
- `issuemap history` — view detailed change history
- `issuemap global` — cross-project operations
- `issuemap guide` — interactive workflow guide

Run `issuemap --help` or `issuemap <command> --help` for full usage.

For a comprehensive workflow guide, see `docs/workflow.md` or run `issuemap guide`.


## HTTP API
When the server is running, the API is available at:
- Base URL: `http://localhost:<port>/api/v1`
- Health: `GET /api/v1/health`
- Info:   `GET /api/v1/info`

The web UI (served on the same port) calls the API for you. It’s handy for quick browsing, searching, and inspecting diffs linked to issues.


## Configuration
- Config directory: `.issuemap/`
- Main config file: `.issuemap/config.yaml`
- Defaults: IDs look like `ISSUE-001`, default priority is `medium`, default status is `open`.

You can keep templates under `.issuemap/templates/` to standardize new issues.


## Development
Clone and build:
```bash
git clone https://github.com/ooyeku/issuemap
cd issuemap
make build         # outputs ./bin/issuemap
```

Run tests:
```bash
make test          # unit + a stable set of integration tests
# or
make test-all      # broader integration suite (can take longer)
```

Useful targets:
- `make install` — compile and install to GOPATH/bin
- `make lint`, `make fmt`, `make vet`
- `make clean`


## Git hygiene (.gitignore)
Most teams commit issue/history/templates but ignore ephemeral server files. This repository includes a `.gitignore` with the following noteworthy entries:
- `.issuemap/server.log`
- `.issuemap/server.pid`
- `.issuemap/metadata/bulk_logs/`
- `bin/` (except `bin/smoke.sh`)


## Roadmap
There’s an in-repo roadmap for a keyboard-first terminal UI (TUI). See `todo.md` for the current plan and acceptance criteria. PRs welcome.


## Contributing
- Discuss significant changes in an issue first when possible.
- Keep changesets small and focused; include tests where it makes sense.
- Be kind in code review.


## License
MIT — see LICENSE for details.
