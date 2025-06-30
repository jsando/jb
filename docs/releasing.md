# Release Process

## Overview

jb uses [GoReleaser](https://goreleaser.com/) to automate the release process. When you push a tag starting with `v`, it will:

1. Build binaries for multiple platforms (Linux, macOS, Windows)
2. Create a GitHub release with the binaries
3. Generate a changelog
4. Create checksums for all artifacts

## Creating a Release

1. Create and push a tag:
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```

2. GoReleaser will automatically:
   - Build binaries for all platforms
   - Inject version information via ldflags
   - Create a GitHub release
   - Upload all artifacts

## Testing Locally

To test the release process locally without creating a release:

```bash
goreleaser build --snapshot --clean
```

## Future: Homebrew Support

Once ready to publish to Homebrew:

1. The tap repository is already created at `jsando/homebrew-tools`
2. Create a GitHub token with repo access to the tap
3. Add the token as `HOMEBREW_TAP_GITHUB_TOKEN` in GitHub secrets
4. Update `.goreleaser.yaml` to include the brew configuration

Users will then be able to install via:
```bash
brew tap jsando/tools
brew install jb
```

## Future: Chocolatey Support

For Windows users, Chocolatey support is configured but disabled by default. To enable:

1. Get a Chocolatey API key
2. Add it as `CHOCOLATEY_API_KEY` in GitHub secrets
3. Set `skip_publish: false` in the chocolateys section of `.goreleaser.yaml`

Users will then be able to install via:
```powershell
choco install jb
```