# stack-demo — builds the web/worker binary (role chosen at runtime by ROLE env).
# The pgx driver is resolved at build time (the sample go.mod leaves it unpinned).
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY main.go .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/app .

FROM gcr.io/distroless/static
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
