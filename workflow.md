## IssueMap Golden Workflow

A simple, opinionated workflow for day‑to‑day development that integrates Git and IssueMap.

Goals
- Keep the mental model small: one issue = one branch = one merge.
- Make essentials obvious; advanced features optional.
- Be easy to automate later via consistent checkpoints.

Who this is for
- Solo devs and teams who want a clear, memorable flow.
- Junior‑friendly; works with any Git hosting.


### The 7‑Step Golden Path

0) Start the server

```sh
issuemap server start
```

1) Project setup (once per repo)
- Initialize Git and IssueMap:

```sh
git init -b main
issuemap init --name "My Project"
```

- Optional: Start the local API server (for dashboards/integrations):

```sh
issuemap server start
  ```

2) Create an issue (one unit of work)
- Create with sensible defaults; add metadata as needed:
```sh
issuemap create "Add login with session cookie" \
  --type feature --priority high --labels auth,backend
# or use a template
issuemap create "Hotfix: CSRF token mismatch" --template hotfix
``` 
- See it:

```sh
issuemap list
```

3) Prepare the branch (exactly one branch per issue)
- Create and switch using the issue ID. Both forms are accepted: `001` or `ISSUE-001`.

```sh
issuemap branch ISSUE-001
```

- This generates `feature/ISSUE-001-<sanitized-title>` and records the branch on the issue.

4) Do the work
- Write code; commit frequently. Include the issue ID in commit messages to auto‑link:

```sh
git add -A
git commit -m "ISSUE-001: implement cookie session login"
```

- Optional time tracking:

```sh
issuemap start ISSUE-001   # start a timer
issuemap stop ISSUE-001    # stop and log time
  ```

5) Keep IssueMap in sync (as you go)
- Refresh derived status, stats, and branch associations:

```sh
issuemap sync --auto-update
```

- Inspect current issue details:

```sh
issuemap show ISSUE-001
```

6) Merge and close (one merge per issue)
- Option A: From the feature branch, auto‑detect the issue:

```sh
issuemap merge
```

- Option B: From `main`, specify the issue to merge its branch into `main`:

```sh
issuemap merge ISSUE-001
```

- The merge command will ensure `.issuemap` files are committed and then close the issue on success.

7) Housekeeping
- Delete the feature branch if desired and confirm status:

```sh
git branch -d feature/ISSUE-001-add-login-with-session-cookie
issuemap list --type feature --status open
```


### Daily Developer Loop (Quick Reference)
- Pick work:

```sh
issuemap list --status open --type feature
```

- Assign and estimate (optional):

```sh
issuemap assign ISSUE-001 me
issuemap estimate ISSUE-001 4h
  ```
- Branch and code:

```sh
issuemap branch ISSUE-001
# edit code
git commit -m "ISSUE-001: first pass"
issuemap sync --auto-update
  ```
- Wrap up:

```sh
issuemap merge        # from feature branch
# or
issuemap merge ISSUE-001   # from main
  ```


### Conventions (sane defaults)
- Branch naming
  - Default: `<prefix>/<ISSUE-ID>-<short-title>`, e.g., `feature/ISSUE-042-add-sso`.
  - Prefix auto‑derives from issue type; configurable in `.issuemap/config.yaml`.

- Commit messages
  - Start with the issue ID: `ISSUE-042: short summary`.
  - This keeps links and history coherent.

- Issue metadata
  - Type: `feature`, `bug`, `task` (defaults apply if omitted).
  - Priority: `low`, `medium`, `high`.
  - Labels: simple strings; use a small, shared vocabulary.
  - Templates: pre‑baked defaults for repeatable work (e.g., `hotfix`, `improvement`).


### Collaboration & Coordination
- Assign work

```sh
issuemap assign ISSUE-101 alice
  ```
- Express order and blockers

```sh
issuemap depend ISSUE-202 --on ISSUE-101
issuemap deps ISSUE-202
  ```
- Track progress and history

```sh
issuemap history ISSUE-101
issuemap report --format table
  ```


### Troubleshooting Tips
- Merge says the worktree has unstaged changes
  - The `merge` command now auto‑commits `.issuemap` changes when needed. If you prefer manual control:

```sh
git add .issuemap && git commit -m "Update issues"
    ```
- Branch or issue not found
  - Use exact IDs: `001` or `ISSUE-001`. Check with `issuemap list`.
- Server 404s in custom tooling
  - Ensure the server is running: `issuemap server`.


### Minimal Cheat Sheet
- Create → Branch → Commit → Sync → Merge → Close

```sh
issuemap create "Title" --type feature --priority medium
issuemap branch ISSUE-123
git commit -m "ISSUE-123: change"
issuemap sync --auto-update
issuemap merge              # on feature branch
# or: issuemap merge ISSUE-123   # on main
```


### Automation Checkpoints (for future tooling)
- After create: auto‑branch and optional auto‑assign to creator.
- On first commit referencing an issue: auto‑start time tracking.
- On PR open/merge: reconcile status and close the issue.
- On merge to `main`: tag release or update changelog using issue metadata.
