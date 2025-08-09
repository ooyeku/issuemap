# IssueMap TUI Roadmap

Professional, keyboard-first terminal UI that achieves feature parity with the CLI while remaining fast, accessible, and Git-native. No emojis.

## Goals

- [ ] Professional look and feel; consistent with existing CLI colors and formatting
- [ ] Keyboard-first, mouse-optional; discoverable help overlay
- [ ] Parity with core CLI flows (create → branch → commit → sync → merge → close)
- [ ] Works connected to `issuemap server` and in offline/file mode
- [ ] Scales to large repos; smooth rendering and minimal flicker

## Framework & Architecture

- [ ] Evaluate TUI frameworks and select one (Bubble Tea vs tview)
- [ ] Architecture doc: app model, state store, actions, view routing
- [ ] Theming and style guide (colors, spacing, tables) aligned with CLI
- [ ] Keybinding map and conventions (Vim-like navigation, Emacs alternates)
- [ ] Error/notification surface strategy (status bar and non-blocking banners)
- [ ] Data layer: client that mirrors CLI services (server-first, file fallback)

## Command Entry Point

- [ ] Add `issuemap tui` command (with `--read-only`, `--server`, `--repo` flags)
- [ ] Detect server and show connection status; auto-fallback to file mode
- [ ] Global help: `?` opens keybindings and command palette

## Core Shell & Navigation

- [ ] Root layout: header (context), left nav, main content, status bar
- [ ] Router: switch between List, Detail, Board, Search, Graph, Activity, Settings
- [ ] Command palette (Ctrl+P) for quick actions and navigation
- [ ] Persist UI state across sessions (last view, filters, columns)

## Issue List View (MVP)

- [ ] Virtualized table with columns: ID, Title, Type, Status, Assignee, Priority, Labels, Updated
- [ ] Sorting (Title/Status/Priority/Updated)
- [ ] Filtering using query DSL (same as CLI); inline query bar
- [ ] Saved searches sidebar (load, rename, delete)
- [ ] Keyboard navigation (j/k, arrows), open detail (Enter), multi-select (Space)
- [ ] Inline quick actions: assign, label, status change, estimate

## Issue Detail View (MVP)

- [ ] Overview: fields (title, type, status, priority, labels, assignee)
- [ ] Edit fields in-place with validation
- [ ] Branch and commit section (show linked branch, recent commits)
- [ ] Time tracking controls (start/stop, show active, edit entries)
- [ ] Checklist (add, check/uncheck, reorder)
- [ ] Dependencies section (blocked by/blocks; quick add/remove)
- [ ] History timeline (status changes, edits)

## Parity Group A (everyday flows)

- [ ] Create issue (form + template selection)
- [ ] Assign/unassign
- [ ] Change status (with project transitions if configured)
- [ ] Update labels
- [ ] Estimate/update estimate
- [ ] Start/stop timer from list and detail
- [ ] Close/reopen issue with reason

## Board (Kanban) View

- [ ] Columns from configured statuses; column order configurable
- [ ] Keyboard drag-and-drop (move between columns, reorder within column)
- [ ] Swimlanes by assignee or label
- [ ] WIP limit indicators per column/user
- [ ] Column-level quick filters

## Search & Filters

- [ ] Full query DSL support with syntax highlighting
- [ ] Saved searches management (create, update, delete)
- [ ] Search results view with same capabilities as Issue List

## Dependencies Graph

- [ ] Inline mini-graph in Issue Detail
- [ ] Full-screen graph view (navigate nodes/edges, open issue)
- [ ] Filters: show only blockers, only downstream, depth limits

## Bulk Operations

- [ ] Multi-select across List and Search results
- [ ] Bulk assign/label/status/priority
- [ ] Bulk preview and confirmation dialog
- [ ] Progress bar and per-item result reporting

## Activity & Notifications

- [ ] Live activity feed (server mode), periodic polling (file mode)
- [ ] Non-blocking status notifications and error surfacing
- [ ] Mention/assignee pings via configured notifiers (future-friendly hook)

## Settings & Customization

- [ ] Theme selector (light/dark/high-contrast); persist to config
- [ ] Configure columns and column widths per view
- [ ] Configure keybindings with defaults and overrides
- [ ] Toggle advanced features (board, graph) on low-power terminals

## Performance & Reliability

- [ ] Incremental rendering and throttled updates under load
- [ ] Background data refresh with cache invalidation
- [ ] Robust terminal size changes; minimal flicker on resize
- [ ] Handle 10k+ issues: pagination/virtualization strategy validated

## Testing & CI

- [ ] Unit tests for state reducers and view models
- [ ] Golden/snapshot tests for rendered views (TTY harness)
- [ ] Key-sequence integration tests for critical flows
- [ ] CI job to run TUI tests in headless environment

## Packaging & Docs

- [ ] Build tags and module split if framework adds weight
- [ ] Cross-platform validation (macOS, Linux, Windows terminals)
- [ ] Man page/help updates for `issuemap tui`
- [ ] User guide: TUI quickstart, keybindings, and tips

---

### Backlog (Power Features)

- [ ] Command macros/recipes inside TUI (chain actions on selection)
- [ ] Inline diff/review panel for recent commits linked to issue
- [ ] Offline change queue visualizer with conflict resolver
- [ ] Export current view (CSV/JSON/Markdown)
- [ ] Calendar panel (due dates, estimates) with ICS export shortcut

### Non-Goals (for now)

- [ ] Mouse-only workflows (keyboard-first remains primary)
- [ ] Emoji indicators

### Acceptance Criteria

- [ ] Core views (List, Detail) complete and round-trip editable
- [ ] Board usable with keyboard-only DnD and WIP signals
- [ ] Query DSL parity with CLI, including saved searches
- [ ] Bulk operations with preview and progress reporting
- [ ] Smooth on 10k issues; no visual tearing; <150ms typical interactions

Last Updated: TODO
