FROM golang:1.26.3-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY service-biz/go.mod service-biz/go.sum ./
RUN go mod download

COPY service-biz/ .

RUN GOOS=linux GOARCH=amd64 go build -o /out/service-biz ./cmd/service-biz

FROM golang:1.26.3-alpine AS relay-builder
WORKDIR /src
COPY server-wire-relay/go.mod ./
RUN go mod download
COPY server-wire-relay/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-relay-amd64 ./cmd/server-wire-relay && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/server-wire-relay-arm64 ./cmd/server-wire-relay

FROM golang:1.26.3-alpine AS punch-builder
WORKDIR /src
COPY server-wire-punch/go.mod ./
RUN go mod download
COPY server-wire-punch/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-punch-amd64 ./cmd/server-wire-punch && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/server-wire-punch-arm64 ./cmd/server-wire-punch

FROM golang:1.26.3-alpine AS derp-builder
WORKDIR /src
COPY server-wire-derp/go.mod ./
RUN go mod download
COPY server-wire-derp/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-derp-amd64 ./cmd/server-wire-derp && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/server-wire-derp-arm64 ./cmd/server-wire-derp

FROM golang:1.26.3-alpine AS edge-proxy-builder
WORKDIR /src
COPY server-edge-proxy/go.mod ./
COPY server-edge-proxy/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-edge-proxy-amd64 ./cmd/server-edge-proxy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/server-edge-proxy-arm64 ./cmd/server-edge-proxy

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

ENV SLAN_SERVICE_BIZ_DB_HOST=postgres
ENV SLAN_SERVICE_BIZ_DB_PORT=5432
ENV SLAN_SERVICE_BIZ_DB_USER=slan
ENV SLAN_SERVICE_BIZ_DB_PASSWORD=slan
ENV SLAN_SERVICE_BIZ_DB_NAME=slan
ENV SLAN_SERVICE_BIZ_DB_SSLMODE=disable
ENV SLAN_SERVICE_BIZ_DB_TIMEZONE=UTC
ENV SLAN_BIZ_ADDR=:8080

COPY --from=builder /out/service-biz /app/service-biz
COPY --from=relay-builder /out/ /opt/slan/node-assets/
COPY --from=punch-builder /out/ /opt/slan/node-assets/
COPY --from=derp-builder /out/ /opt/slan/node-assets/
COPY --from=edge-proxy-builder /out/ /opt/slan/node-assets/

EXPOSE 8080

ENTRYPOINT ["/app/service-biz"]
