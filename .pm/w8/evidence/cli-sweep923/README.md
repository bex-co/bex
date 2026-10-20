# CLI sweep 923 — repo/branch PATCH on an image-backed service (live, 2026-10-20)

Disposable free image-backed web service created, source-patched, deleted (204, then GET 404). No new bug filed.

- `PATCH {"repo":"https://github.com/bex-co/bex"}` and `{"branch":"qa923-branch"}` on an image-backed service → 200, both read back — source settings are writable post-create (the source-conversion direction the PATCH fold's repo/branch fields exist for), while the running image association is untouched until a deploy acts on it. Contrast sweep 875: build-only *build* fields (rootDir/buildFilter) refuse on a repo-less service, but setting the repo itself is the legitimate first step and is not gated.
