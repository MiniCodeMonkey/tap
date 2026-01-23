#!/bin/bash
# Ralph Wiggum - Long-running AI agent loop
# Usage: ./ralph.sh [max_iterations]
#
# Uses stream-json output format to reliably detect when Claude finishes
# (workaround for https://github.com/anthropics/claude-code/issues/19060)

set -e

# Parse arguments
MAX_ITERATIONS=${1:-10}

# Find work directory: use ./ralph/ subdirectory if it exists, otherwise current dir
if [ -d "$(pwd)/ralph" ] && [ -f "$(pwd)/ralph/prd.json" ]; then
  WORK_DIR="$(pwd)/ralph"
else
  WORK_DIR="$(pwd)"
fi
PRD_FILE="$WORK_DIR/prd.json"
PROGRESS_FILE="$WORK_DIR/progress.txt"
ARCHIVE_DIR="$WORK_DIR/archive"
LAST_BRANCH_FILE="$WORK_DIR/.last-branch"
CLAUDE_MD="$WORK_DIR/CLAUDE.md"

# Create temp file for capturing output
TEMP_OUTPUT=$(mktemp)
trap "rm -f $TEMP_OUTPUT" EXIT

# Archive previous run if branch changed
if [ -f "$PRD_FILE" ] && [ -f "$LAST_BRANCH_FILE" ]; then
  CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
  LAST_BRANCH=$(cat "$LAST_BRANCH_FILE" 2>/dev/null || echo "")

  if [ -n "$CURRENT_BRANCH" ] && [ -n "$LAST_BRANCH" ] && [ "$CURRENT_BRANCH" != "$LAST_BRANCH" ]; then
    # Archive the previous run
    DATE=$(date +%Y-%m-%d)
    # Strip "ralph/" prefix from branch name for folder
    FOLDER_NAME=$(echo "$LAST_BRANCH" | sed 's|^ralph/||')
    ARCHIVE_FOLDER="$ARCHIVE_DIR/$DATE-$FOLDER_NAME"

    echo "Archiving previous run: $LAST_BRANCH"
    mkdir -p "$ARCHIVE_FOLDER"
    [ -f "$PRD_FILE" ] && cp "$PRD_FILE" "$ARCHIVE_FOLDER/"
    [ -f "$PROGRESS_FILE" ] && cp "$PROGRESS_FILE" "$ARCHIVE_FOLDER/"
    echo "   Archived to: $ARCHIVE_FOLDER"

    # Reset progress file for new run
    echo "# Ralph Progress Log" > "$PROGRESS_FILE"
    echo "Started: $(date)" >> "$PROGRESS_FILE"
    echo "---" >> "$PROGRESS_FILE"
  fi
fi

# Track current branch
if [ -f "$PRD_FILE" ]; then
  CURRENT_BRANCH=$(jq -r '.branchName // empty' "$PRD_FILE" 2>/dev/null || echo "")
  if [ -n "$CURRENT_BRANCH" ]; then
    echo "$CURRENT_BRANCH" > "$LAST_BRANCH_FILE"
  fi
fi

# Initialize progress file if it doesn't exist
if [ ! -f "$PROGRESS_FILE" ]; then
  echo "# Ralph Progress Log" > "$PROGRESS_FILE"
  echo "Started: $(date)" >> "$PROGRESS_FILE"
  echo "---" >> "$PROGRESS_FILE"
fi

# Verify required files exist
if [ ! -f "$PRD_FILE" ]; then
  echo "Error: prd.json not found in $WORK_DIR"
  echo "Ralph requires prd.json and CLAUDE.md in the current directory."
  exit 1
fi

echo "Starting Ralph in $WORK_DIR"
echo "Max iterations: $MAX_ITERATIONS"

for i in $(seq 1 $MAX_ITERATIONS); do
  echo ""
  echo "==============================================================="
  echo "  Ralph Iteration $i of $MAX_ITERATIONS"
  echo "==============================================================="

  # Read prompt
  if [ ! -f "$CLAUDE_MD" ]; then
    echo "Error: CLAUDE.md not found in $WORK_DIR"
    exit 1
  fi
  PROMPT="$(cat "$CLAUDE_MD")"
  > "$TEMP_OUTPUT"

  # Run claude in background with stream-json for reliable completion detection
  claude --dangerously-skip-permissions -p "$PROMPT" --output-format stream-json > "$TEMP_OUTPUT" 2>&1 &
  CLAUDE_PID=$!

  # Tail output in background to show progress
  tail -f "$TEMP_OUTPUT" 2>/dev/null &
  TAIL_PID=$!

  # Monitor for result marker (indicates Claude finished)
  RESULT_RECEIVED=false
  while kill -0 $CLAUDE_PID 2>/dev/null; do
    if grep -q '"type":"result"' "$TEMP_OUTPUT" 2>/dev/null; then
      RESULT_RECEIVED=true
      sleep 1  # Let it finish writing
      kill $CLAUDE_PID 2>/dev/null || true
      break
    fi
    sleep 0.5
  done

  # Clean up
  kill $TAIL_PID 2>/dev/null || true
  wait $CLAUDE_PID 2>/dev/null || true

  # Show result summary
  if [ "$RESULT_RECEIVED" = true ]; then
    echo ""
    echo "Session completed (detected via stream-json)"
  else
    echo ""
    echo "Warning: No result marker received, continuing anyway..."
  fi

  # Check for completion signal
  if grep -q "<promise>COMPLETE</promise>" "$TEMP_OUTPUT" 2>/dev/null; then
    echo ""
    echo "Ralph completed all tasks!"
    echo "Completed at iteration $i of $MAX_ITERATIONS"
    exit 0
  fi

  echo "Iteration $i complete. Continuing..."
  sleep 2
done

echo ""
echo "Ralph reached max iterations ($MAX_ITERATIONS) without completing all tasks."
echo "Check $PROGRESS_FILE for status."
exit 1
