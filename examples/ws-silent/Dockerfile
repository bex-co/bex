# Deterministic build (the bex builder's "auto" mode uses a Dockerfile when present,
# else Cloud Native Buildpacks). Multi-stage → tiny static image.
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /app .

# Needs a shell: OpenSandbox injects a bootstrap entrypoint that requires /bin/sh,
# so a distroless base won't start. debian-slim is small and has a shell.
FROM debian:bookworm-slim
COPY --from=build /app /app
ENV PORT=3000
ENTRYPOINT ["/app"]
