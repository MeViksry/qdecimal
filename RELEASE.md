# qdecimal Release Process

This repository keeps `qdecimal` as a standalone Go module at the repository root.

## Automated Releases

The `qdecimal Release` workflow runs on self-hosted runners.

- Pushes to `main` publish or update the `nightly` prerelease.
- Tags matching `vMAJOR.MINOR.PATCH` publish versioned qdecimal source archives
  and prime the Go module proxy for pkg.go.dev discovery.
- Prerelease tags are supported with `vMAJOR.MINOR.PATCH-prerelease`.

Each release runs formatting checks, dependency-policy checks, unit tests, race
tests, coverage with a minimum threshold, stress tests, fuzz smoke tests,
benchmark smoke tests, `go vet`, `govulncheck`, and `go build` before
publishing.
The publish job uses the checked-in `internal/releasegithub` helper with a
job-scoped `contents: write` token instead of a third-party release action, so
self-hosted runners do not execute external release-publishing action code while
holding write credentials. CI and release jobs also define explicit timeouts to
avoid stuck self-hosted jobs.
The equivalent local gate is:

```bash
make audit
```

To validate the release archive and checksum format without publishing:

```bash
make release-dry-run RELEASE_VERSION=v0.1.0
```

To compile the GitHub release publisher without contacting GitHub:

```bash
make release-helper-check
```

Release assets contain:

- a `qdecimal-<version>.tar.gz` source archive;
- `checksums.txt` with SHA-256 checksums.

## Tagging

Because this module lives at `github.com/MeViksry/qdecimal`, release tags use
the standard Go module format:

```bash
git tag v0.1.0
git push origin v0.1.0
```

After a tagged release, the workflow runs:

```bash
GOPROXY=https://proxy.golang.org,direct go list -m github.com/MeViksry/qdecimal@v0.1.0
```

That is the standard Go package publication path for `go get` and pkg.go.dev.
GitHub Packages is intentionally not used for this Go library.
