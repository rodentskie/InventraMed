# Complete Action

1. Stage all changes and commit with a descriptive message
2. Mark @context/current-feature.md status to `complete`
3. Reset current-feature.md:
    - Change H1 back to `# Current Feature`
    - Clear Goals and Notes sections (keep placeholder comments)
    - single main bullet
    - sub bullets if neede for details
4. Add feature summary to the END of History @context/history.md
5. Push to remote origin then create a `pull request`, use `gh` cli
    - this is multi tenant and every tenant has different main branch (main-**name**) format, `main` is just the base branch now for a new tenant, keep that in mind when creating a PR
    - always check where is the `feature` or `fix` branch is branched out
    - use @.github/pull_request_template.md for template