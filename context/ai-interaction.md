# AI Interaction Guidelines

## Communication

- Be concise and direct
- Explain non-obvious decisions briefly
- Ask before large refactors or architectural changes
- Don't add features not in the project spec
- Never delete files without clarification

## Workflow

This is the common workflow that we will use for every single feature/fix:

1. **Document** - Document the feature in @context/current-feature.md.
2. **Branch** - Create new branch for feature, fix, etc
3. **Implement** - Implement the feature/fix that I create in @context/current-feature.md
4. **Test** - Run tidy, lint, and build via Nx. Fix any failures before proceeding.
   - **apps:** `TASK=tidy APP=<app> pnpm run nx-run` → `TASK=lint APP=<app> pnpm run nx-run` → `TASK=build APP=<app> pnpm run nx-run`
   - Run `TASK=test APP=<app> pnpm run nx-run` for unit tests when applicable
5. **Iterate** - Iterate and change things if needed
6. **Commit** - Ask before committing. Stage relevant files and commit with a conventional message
7. **Pull Request** - Push branch and open a PR via `gh pr create` using `.github/pull_request_template.md`
8. **Merge** - Merge the PR on GitHub after review (User will manually merge)
9. **Review** - Review AI-generated code periodically and on demand.
10. Mark as completed in @context/current-feature.md and add to history at @context/history.md

Do NOT commit without permission and until the build and tests pass. If build or tests fail, fix the issues first.

## Branching

We will create a new branch for every feature/fix. Name branch **feature/[feature]** or **fix[fix]**, etc. Do not delete branch, I will manually delete when I manually run `git fetch --prune`.

## Commits

- Ask before committing (don't auto-commit)
- Use conventional commit messages (feat:, fix:, chore:, etc.)
- Keep commits focused (one feature/fix per commit)
- Never put "Generated With Claude" in the commit messages

## When Stuck

- If something isn't working after 2-3 attempts, stop and explain the issue
- Don't keep trying random fixes
- Ask for clarification if requirements are unclear

## Code Changes

- Make minimal changes to accomplish the task
- Don't refactor unrelated code unless asked
- Don't add "nice to have" features
- Preserve existing patterns in the codebase

## Code Review

Review AI-generated code periodically, especially for:

- Security (auth checks, input validation)
- Performance (unnecessary re-renders, N+1 queries)
- Logic errors (edge cases)
- Patterns (matches existing codebase?)
