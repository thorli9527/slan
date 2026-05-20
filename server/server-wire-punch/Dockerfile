FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-punch ./cmd/server-wire-punch

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/server-wire-punch /app/server-wire-punch

EXPOSE 29130/udp 29131

ENTRYPOINT ["/app/server-wire-punch"]
