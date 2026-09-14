# Working on Yogilib

Yogilib is a Go web app for the works of Yogi Narharinath. Read README.md and the relevant docs before changing it. Explain outcomes in simple language, make routine reversible choices, and preserve the user's existing work.

## Setup and checks

- Use Go 1.25 or newer, as required by go.mod. Download dependencies with `go mod download`.
- Run `go test ./...` and `go vet ./...` for code changes. A database is not needed for the default unit tests. Report skipped database checks honestly.
- For database-free checks, unset `TEST_DATABASE_URL` and `YOGILIB_READER_TEST`; these opt into database tests. Do not source .env.local just to run unit tests.
- Running the app needs DATABASE_URL and creates schema/seeds on startup. Use a separate development database for work that writes data or changes schema. Never treat shared live data as disposable test data.
- Keep credentials in ignored local environment files or the environment; never include their values in commits, logs, or handoff notes.

## Continuing across devices

- Follow docs/07-cross-device-work.md. Prefer continuing the same task on the same connected host when possible.
- Before syncing code, check the current branch, uncommitted changes, and current remote state. Preserve changes from other devices. Never force-push or reset work to resolve an ordinary sync issue.
- When handing work over, report the branch and commit, changes made, checks run, and what remains. Say explicitly if changes have not been pushed.
- Git transfers tracked project files. It does not transfer .env.local, uploaded files under static/docs, database data, or automatically continue conversations.
- Do not claim another device is connected until pairing and a task continuation have been verified. Do not deploy as part of ordinary device setup.
