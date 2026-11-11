# Releasing the Go module

This repository publishes a Go library. The versioned Git tag is the source
consumed by the Go toolchain; GitHub Releases carry the changelog. There are no
binary archives to rebuild or upload.

1. Inspect all worktrees and the remote branch, tags and releases. Preserve
   unrelated edits and choose an unused semantic version. Update `CHANGELOG.md`
   and the README installation version.
2. Commit the intended files on a branch, open a pull request, and merge only after
   its checks succeed. Wait for every CI job to succeed for that exact commit,
   including race tests on Linux/macOS, vet, lint, vulnerability checks and the
   pinned Netdisco/PostgreSQL integration.
3. Create an annotated `vMAJOR.MINOR.PATCH` tag at the validated commit and push
   the tag. Create the GitHub Release with explicit release notes and verify
   its tag and target. Never move a published version tag.
4. In a temporary directory outside the repository, use `GOWORK=off` and a
   versioned `require` with no `replace`. Download through the public Go proxy
   and checksum database, compile/use the public interface, and verify module
   checksums and the downloaded module contents against the tagged source.
5. Read back the tag, Release, CI result, remote/main alignment and worktree
   status. Record the published URL and install command in the delivery reply.

If a version or draft already exists, inspect it before retrying publication.
A failed or queued workflow is not a successful release gate. Repository
visibility is independent of version publication.
