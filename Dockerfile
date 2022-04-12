FROM golang:1.18-alpine3.15 as builder

WORKDIR /src

COPY ["go.mod", "go.sum", "./"]

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' -a \
    -o /prometheus-filter-proxy .

FROM gcr.io/distroless/static AS final

USER nonroot:nonroot

COPY --from=builder --chown=nonroot:nonroot /prometheus-filter-proxy /prometheus-filter-proxy

ENTRYPOINT ["/prometheus-filter-proxy"]
