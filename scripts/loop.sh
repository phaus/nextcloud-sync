#!/bin/bash
# -m "opencode/claude-opus-4-5" "
while :; do opencode run "READ all of README.md. READ all of AGENTS.md. READ all of implementation-plan.md. Pick ONE task. Verify via web/code search. Complete task, verify via CLI/Test output. Commit change. ONLY do one task. Update plan.md. If you learn a critical operational detail (e.g. how to build), update AGENTS.md. Say 'I am done', when you think, your are done! If all tasks done, sleep 5s and exit. NEVER GIT PUSH. ONLY COMMIT."; done

