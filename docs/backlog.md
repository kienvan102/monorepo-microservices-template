# Backlog

Outstanding issues for the project, ordered by priority. Each entry has: what the problem is, its impact, and how to address it. Once an entry is resolved, delete it from this file.

## High

### Missing automated tests

- **Problem:** `commonlib` (`config`, `logger`, `mongoclient`, `processor`) and `core/app` have tests; everything else — both `services/mongoanalyzer` and `cmd/db-analyzer` — still has to be checked by hand against a real MongoDB instance.
- **Impact:** a code change can easily break existing behavior without anyone noticing.
- **How to address it:** write unit tests for the parts that don't need a real DB:
  - `jsrunner/catalog.go`: reading the plan, validating script names, checking a script matches its collection.
  - `jsrunner/runner.go`: splitting `ANALYZER_RESULT_JSON:` and `ANALYZER_PROGRESS_JSON:` lines out of `mongosh`'s output, redacting the URI from output and errors.
  - `transport/cli`: action dispatch, error messages for an invalid action or a script that doesn't exist.

## Medium

### Docker image has never been built

- **Problem:** `cmd/db-analyzer/Dockerfile` has never been built, since the current development machine doesn't have Docker permissions (the user isn't in the `docker` group).
- **Impact:** the image-based deploy path is unverified: installing `mongosh`, running as a non-root user, reading configuration from environment variables.
- **How to address it:** get Docker permissions, then run `make db-analyzer-image` and `docker run --env-file cmd/db-analyzer/.env db-analyzer -action check`. Confirm the image doesn't contain `.env`.

## Low

### Error message points at a command that doesn't exist

- **Problem:** when `-action run` is given a script name that doesn't exist, the printed error is `use make list to see available scripts`. The correct target is `make db-analyzer-list`.
- **How to address it:** fix the message in `services/mongoanalyzer/transport/cli/cli.go`. Better yet, don't mention `make` at all, since the transport layer doesn't know how the deployment is actually run.

### Script validation is duplicated between business and jsrunner

- **Problem:** `Analyzer.RunScripts` validates `catalog.Exists`/`catalog.ValidateTarget` for every selected name before running anything (`business/analyzer.go:62-67`), and `Runner.Run` performs the identical two checks again internally for each script (`jsrunner/runner.go:37-42`). Neither layer documents that it's deliberately re-checking what the other already checked.
- **Impact:** not a correctness bug — both checks agree — but it's redundant work on every script run, and it's unclear whether `Runner` re-validates on purpose (defending itself as a port other callers could use directly) or whether this is accidental drift. A future change to the validation rule has two places to update and no clear owner.
- **How to address it:** decide explicitly which layer owns validation. If `Runner.Run` re-validates on purpose (since `ScriptRunner` is a general-purpose port, not guaranteed to be called only through `Analyzer`), say so in a comment and drop the redundant checks from `Analyzer.RunScripts`, keeping only its actual fail-fast-across-the-whole-batch concern there (checking all selected names before running any of them, so a bad name late in the list doesn't waste earlier runs).
