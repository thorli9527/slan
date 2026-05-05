FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-relay ./cmd/server-wire-relay

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/server-wire-relay /app/server-wire-relay

EXPOSE 29110/udp 29111

ENTRYPOINT ["/app/server-wire-relay"]
