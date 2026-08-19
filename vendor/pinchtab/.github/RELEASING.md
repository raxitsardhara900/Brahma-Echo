# Releasing PinchTab

This is the release checklist for the GitHub tag-driven release pipeline.

## One-time setup

- Create the public tap repository `pinchtab/homebrew-tap`.
- Add the `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret to `pinchtab/pinchtab`.
- The token must have write access to `pinchtab/homebrew-tap`.

## What the release workflow does

Pushing a tag like `v0.7.0` triggers [release.yml](/Users/luigi/dev/prj/giago/pt-bosch/.github/workflows/release.yml), which:

1. Builds release binaries and creates the GitHub release via GoReleaser.
2. Publishes the npm package.
3. Builds and publishes container images.
4. Generates a Homebrew formula PR against `pinchtab/homebrew-tap`.

## Release steps

1. Verify the branch you want to release is merged to `main`.
2. If the release includes security-impacting changes, update `skills/pinchtab/TRUST.md`:
   - extend `## Security History` when needed
   - adjust `**Minimum recommended version:**` if the safe floor changes
3. Push the release tag:

```bash
git tag v0.7.0
git push origin v0.7.0
```

4. Watch the `Release` workflow in GitHub Actions.
5. Confirm GoReleaser opens a PR in `pinchtab/homebrew-tap`.
6. Merge that PR.

After the tap PR is merged, users can install with:

```bash
brew install pinchtab/tap/pinchtab
```

## Notes

- The Homebrew formula is not published until the tap PR is merged.
- No extra automation is required in `homebrew-tap` unless you want auto-merge.
- The release workflow can also be run manually with `workflow_dispatch` and a tag input.
