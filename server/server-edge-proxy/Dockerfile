FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-edge-proxy ./cmd/server-edge-proxy

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/server-edge-proxy /app/server-edge-proxy

EXPOSE 28080 1883

ENTRYPOINT ["/app/server-edge-proxy"]
