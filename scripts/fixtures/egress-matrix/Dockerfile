FROM golang:1.25.6-alpine AS build
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /egress-fixture main.go

FROM scratch
COPY --from=build /egress-fixture /egress-fixture
ENTRYPOINT ["/egress-fixture"]
