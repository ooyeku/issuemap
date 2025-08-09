## IssueMap TUI Quickstart

Audience: developers and automation agents who prefer a keyboard-first terminal UI with parity to CLI flows.

### Launch

```
issuemap tui                 # preview overlay and view routing
issuemap tui --help-overlay  # compact keybindings
issuemap tui --check-parity  # check CLI feature readiness
```

### Views

- list: filter and render issues table
- detail: show one issue (set env `ISSUE_ID=ISSUE-XXX`)
- activity: show recent history entries
- board/search/graph/settings: reserved; progressive rollout

Examples:

```
issuemap tui --view list --status open --labels tui --limit 20
ISSUE_ID=ISSUE-010 issuemap tui --view detail --history --deps --checklist
issuemap tui --view activity --history-limit 10 --recent-days 7
```

### List filters and pagination

```
--status <s>        --assignee <u>        --labels a,b
--limit N           --offset N
--per-page N        --page P   # 1-based; overrides limit/offset
```

### Settings & customization

Persisted to `.issuemap/tui_config.json`:

```
--set-theme light|dark|high-contrast
--set-columns ID,Title,Status,Updated
--set-widths Title=40,ID=10,Status=10,Updated=16
--key <action=keys>             # repeatable
--toggle-feature <name=on|off>  # repeatable; e.g. board=off
--config-only                   # apply config and exit
```

### Server integration

TUI auto-detects a running local server via `.issuemap/server.pid` and `.issuemap/server.log` and shows the connected port when healthy.

Start/stop/status:

```
issuemap server start
issuemap server status
issuemap server stop
```

### Keyboard cheat sheet (planned)

```
j/k or arrows  navigate
enter         open details
space         multi-select
/             focus query
ctrl+p        palette
?             help overlay
```

### Cross‑platform notes

- Tested on macOS/Linux terminals; Windows terminals should work with default fonts and UTF‑8.
- Use `--no-color` if terminal does not render colors properly.

### Snapshots & CI

`scripts/test_tui_snapshots.sh` runs stable TUI commands and compares against snapshots under `test/snapshots/expected/`.


