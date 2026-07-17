# Multi-stage build → minimal runtime image. The job binary exits immediately
# after logging; no shell or package manager needed at runtime.
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /cron-demo .

FROM debian:bookworm-slim
COPY --from=build /cron-demo /cron-demo
ENTRYPOINT ["/cron-demo"]
