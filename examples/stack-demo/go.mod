module bex.co/stack-demo

go 1.26

// The pgx driver (github.com/jackc/pgx/v5/stdlib) is resolved by `go mod tidy`
// at build time (see Dockerfile) — no version pinned here so the sample always
// pulls a buildable release against the toolchain in the build image.
