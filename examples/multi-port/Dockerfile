# The w6/m148 multi-port fixture. The private service ignores $PORT and listens
# on the two TCP ports declared below, which bex publishes under its stable
# hostname, as Render does for private services.
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /app .

# Needs a shell: OpenSandbox injects a bootstrap entrypoint that requires /bin/sh,
# so a distroless base won't start. debian-slim is small and has a shell.
FROM debian:bookworm-slim
COPY --from=build /app /app
USER 65532:65532
EXPOSE 8123 9000
ENTRYPOINT ["/app"]
