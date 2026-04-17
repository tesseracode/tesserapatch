#!/usr/bin/env bash
# Setup script for the reconciliation bug bash.
# Clones the target repo, initializes tpatch, adds two features.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TPATCH="${SCRIPT_DIR}/../tpatch"
WORK_DIR="${SCRIPT_DIR}/copilot-api-work"
PINNED_COMMIT="0ea08febdd7e3e055b03dd298bf57e669500b5c1"
REPO_URL="https://github.com/tesserabox/copilot-api.git"

echo "=== Tessera Patch Reconciliation Bug Bash Setup ==="
echo ""

# Build tpatch
echo "1. Building tpatch..."
cd "${SCRIPT_DIR}/.."
go build -o tpatch ./cmd/tpatch
echo "   Built: ${TPATCH}"
echo ""

# Clone and pin
if [ -d "$WORK_DIR" ]; then
    echo "2. Removing existing work directory..."
    rm -rf "$WORK_DIR"
fi
echo "2. Cloning copilot-api at pinned commit..."
git clone "$REPO_URL" "$WORK_DIR"
cd "$WORK_DIR"
git checkout "$PINNED_COMMIT" 2>/dev/null || git checkout -b bugbash "$PINNED_COMMIT"
echo "   Pinned to: $(git rev-parse HEAD)"
echo ""

# Initialize tpatch
echo "3. Initializing tpatch..."
"$TPATCH" init --path "$WORK_DIR"
echo ""

# Add Feature A: Model translation fix
echo "4. Adding Feature A: Model ID translation fix..."
"$TPATCH" add --path "$WORK_DIR" "Fix model ID translation bug where Claude Code 1m suffix gets stripped causing silent loss of 1-million-token context window" --slug model-id-translation-fix
echo ""

# Add Feature B: Models CLI subcommand
echo "5. Adding Feature B: Models CLI subcommand..."
"$TPATCH" add --path "$WORK_DIR" "Add a models CLI subcommand that lists available models with their display names and context window sizes" --slug models-cli-subcommand
echo ""

# Run analysis (heuristic mode)
echo "6. Running analysis for both features..."
"$TPATCH" analyze --path "$WORK_DIR" model-id-translation-fix
"$TPATCH" analyze --path "$WORK_DIR" models-cli-subcommand
echo ""

# Show status
echo "7. Current status:"
"$TPATCH" status --path "$WORK_DIR"
echo ""

echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Implement Feature A and Feature B in ${WORK_DIR}"
echo "  2. Run: $TPATCH apply --path $WORK_DIR model-id-translation-fix --mode done"
echo "  3. Run: $TPATCH record --path $WORK_DIR model-id-translation-fix"
echo "  4. Run: $TPATCH apply --path $WORK_DIR models-cli-subcommand --mode done"
echo "  5. Run: $TPATCH record --path $WORK_DIR models-cli-subcommand"
echo "  6. Verify: cd $WORK_DIR && bun test && bun run typecheck"
echo "  7. Simulate upstream update and reconcile"
