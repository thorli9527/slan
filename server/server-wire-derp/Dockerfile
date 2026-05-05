FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server-wire-derp ./cmd/server-wire-derp

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/server-wire-derp /app/server-wire-derp

EXPOSE 29120 29121

ENTRYPOINT ["/app/server-wire-derp"]
