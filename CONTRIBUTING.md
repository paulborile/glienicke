# Contributing to Glienicke

## License

Glienicke is licensed under the GNU Affero General Public License v3.0
(AGPLv3). See `LICENCE` for the full text.

## Contributor License Agreement

Before a contribution (pull request, patch, or other submitted material)
can be merged, you must agree to the project's Contributor License
Agreement (CLA), which assigns copyright in your contribution to the
Project Owner (Paul Stephen Borile). This allows the project to be
distributed under AGPLv3 while keeping the option open to offer the
project under additional license terms (e.g. a commercial license) in the
future — a common model for AGPL-licensed open-source projects.

Read the full agreement in `CLA.md` before contributing.

**To sign the CLA:** add your name, GitHub username, and today's date to
`CONTRIBUTORS.md` in your pull request, with a note in the PR description
stating you have read and agree to `CLA.md`. First-time contributors will
be asked to do this if it's missing; maintainers will not merge a PR until
it's signed.

If you are contributing on behalf of an employer, see `CLA.md` section 9
for what's needed in addition to your own signature.

## Making a change

1. Open an issue first for anything beyond a small fix, so the approach can
   be discussed before you invest time in an implementation.
2. Fork the repo and create a branch from `main`.
3. Keep changes focused — one logical change per pull request.
4. Add or update tests for any behavior change.
5. Run before submitting:
   ```bash
   go build ./...
   go vet ./...
   go test ./...
   gofmt -l .   # should print nothing
   ```
6. Update `CHANGELOG.md` for any user-visible change, following semantic
   versioning (see root `CLAUDE.md` / project conventions).
7. Open the pull request, sign the CLA if you haven't already, and describe
   what changed and why.

## Code of conduct

Be respectful and constructive in all project spaces (issues, pull
requests, discussions). Disagreement on technical approach is fine and
expected; personal attacks are not.
