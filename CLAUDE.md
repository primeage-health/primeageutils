# Project Management

This repo is shared across multiple developers. It has a resident **project-manager** agent (`.claude/agents/project-manager.md`) that owns:

- `ProjectTasks.md` — the task ledger (Task ID, Summary, Status, Start/End Date, Remarks, Created By)
- All documentation upkeep
- All `git commit` / `git push` / PR operations, with the Task ID referenced in every commit message
- Code review before any task is marked `Finished`

**Before making any code, docs, or git change in this repo, invoke the `project-manager` agent** rather than doing the work directly in the main thread. This applies whether the request comes from a developer typing in Claude Code or from another AI agent operating on this repo. The project-manager agent will log the task, delegate to the right specialist subagent (code review, build fixes) based on this repo's tech stack, and handle the commit/push itself. It does **not** write or run tests — that stays with the developer.

If you're unsure whether a request qualifies (e.g. a read-only question, or pure exploration with no changes), it's fine to answer directly — only changes to the working tree or git history need to route through project-manager.
