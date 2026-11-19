# Contributing

## Branches

Never commit to `main` directly. Create a branch from the latest `main` named
`<type>/<short-description>`, for example `feat/log-retention` or `fix/newsletter-key`. Every
change reaches `main` through a pull request; merging to `main` deploys to production.

## Commit messages

Commits follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<optional scope>)<optional !>: <description>

<optional body>

<optional footer>
```

- **type** is one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`,
  `chore`, `revert`.
- **scope** names the area changed, in lowercase: `social`, `newsletter`, `web`, `admin`, `api`,
  `ci`, `retention`.
- **description** is imperative and lowercase ("add log retention setting", not "Added …"), has no
  trailing full stop and keeps the header at 72 characters or fewer.
- The **body** explains what changed and why, wrapped at about 72 characters.
- A breaking change adds `!` after the type or scope and a `BREAKING CHANGE:` footer.
- No `Co-authored-by` trailers or tool-generated signatures or footers.

Enable the repository hooks once per clone, so `git commit` rejects malformed messages and
`git push` runs the full checks:

```sh
git config core.hooksPath .githooks
```

## Pull requests

- The PR title follows the same Conventional Commits format; it becomes the squash-merge commit.
- The description says what changed, why, and how it was verified (tests, local run, screenshots).
- Keep a PR to one concern. Squash-merge only after `pnpm check` passes locally.
- No co-author credits or tool-generated signatures in titles, descriptions or comments.

## What to commit

Commit only what the product needs: source, migrations, tests, product configuration and
documentation. Never commit secrets (`.env`), local data (`.data/`, dumps), build output, personal
editor or tool configuration, or local session files. Review `git diff --cached` before every
commit; ignore rules do not protect files that are already tracked.

## Checks before a PR

GitHub Actions runs no tests: merging to `main` only builds and deploys. Every check runs locally,
and the pre-push hook runs them on each push:

```sh
pnpm check
```

It checks the branch's commit messages, runs `go vet` and the Go tests, typechecks and tests the
front-ends, and compiles the release binaries and bundles.

Database integration tests run when `SNAP_TEST_DATABASE_URL` points at a migrated test database.
