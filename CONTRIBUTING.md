# Contributing

English (canonical). Portuguese mirror: [CONTRIBUTING.pt_BR.md](CONTRIBUTING.pt_BR.md).

The bridge is in a controlled development phase. Bug reports and reviewed ideas are welcome. Read this before opening a pull request.

## Issue first

Search the open issues before filing a new one.

- A bug is an issue, one defect per issue, with a reproduction: version, command, what you expected, and what happened. Logs must not contain tokens, prompts, or home-directory paths.
- An idea starts in a GitHub Discussion when Discussions are enabled. Use the idea template when a maintainer asks for an issue. Do not open a pull request from an idea that has not been accepted.
- AI-assisted reports and patches are welcome when the author has verified each one. Do not post an unverified batch.

## How many at a time

Keep a few issues and pull requests open at a time. Finish the ones already in review before opening more. Maintainers may ask you to split or hold a submission.

## Pull request

Link the parent issue in the pull request body. Say what changed and why. Paste the output of `make check`. Before a pull request also run `make e2e` and `make mutate`, and paste those outputs. Name the risks.

A clone does not install the git hooks. Run `make hooks` once, then `make check`, before the pull request. Skipping a hook is forbidden.

Stay inside the issue. Do not mix an unrelated fix, a formatting sweep, or a new dependency into the same pull request.

## Validation

From the repository root:

1. `make hooks` once per clone.
2. `make check` before every commit and again before the pull request.
3. `make e2e` and `make mutate` before the pull request. `make mutate` is slow and is not part of the pre-commit hook.

`make check` is the definition of done. A future CI job calls that target and does not grow a second list.

## Commits and merge

Sign every commit. Do not add a co-author trailer. The commit message describes why the change exists.

`Refs` and a GitHub closing keyword belong in the pull request body, never in the commit message.

Merge with a merge commit so the original commits stay on the branch. Squash merge and rebase merge are refused.

## Documentation

User-facing docs exist in English and Portuguese (`*.md` and `*.pt_BR.md`). The two files have the same number of `##` headings and the same number of table rows. A change to one updates the other in the same commit.

## Security reports

Do not file a public issue for a vulnerability. Open a private security advisory on this repository and wait for a maintainer.
