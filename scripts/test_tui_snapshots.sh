#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BIN="$ROOT_DIR/bin/issuemap"
SNAP_DIR="$ROOT_DIR/test/snapshots"
RUN_DIR="$SNAP_DIR/run"
EXP_DIR="$SNAP_DIR/expected"

mkdir -p "$RUN_DIR" "$EXP_DIR"

# Ensure built binary exists
make -C "$ROOT_DIR" build >/dev/null

# 1) Help overlay (static)
"$BIN" tui --help-overlay >"$RUN_DIR/help_overlay.txt"

# 2) Parity check (mostly static)
"$BIN" tui --check-parity >"$RUN_DIR/check_parity.txt"

# 3) List table header snapshot (stable subset)
"$BIN" tui --config-only --set-columns ID,Title,Status,Updated --set-widths ID=10,Title=20,Status=10,Updated=16 >/dev/null
"$BIN" tui --view list --status open --labels tui --limit 2 | sed -n '1,3p' >"$RUN_DIR/list_header.txt"

# Initialize expected snapshots if missing (first run creates baseline)
if [[ ! -f "$EXP_DIR/help_overlay.txt" ]]; then cp "$RUN_DIR/help_overlay.txt" "$EXP_DIR/help_overlay.txt"; fi
if [[ ! -f "$EXP_DIR/check_parity.txt" ]]; then cp "$RUN_DIR/check_parity.txt" "$EXP_DIR/check_parity.txt"; fi
if [[ ! -f "$EXP_DIR/list_header.txt" ]]; then cp "$RUN_DIR/list_header.txt" "$EXP_DIR/list_header.txt"; fi

# Diff run vs expected
fail=0
for f in help_overlay.txt check_parity.txt list_header.txt; do
  if ! diff -u "$EXP_DIR/$f" "$RUN_DIR/$f" >/dev/null; then
    echo "Snapshot mismatch: $f" >&2
    diff -u "$EXP_DIR/$f" "$RUN_DIR/$f" || true
    fail=1
  fi
done

if [[ $fail -ne 0 ]]; then
  echo "Snapshot tests failed" >&2
  exit 1
fi

echo "Snapshot tests passed"


