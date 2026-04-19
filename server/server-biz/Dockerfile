FROM golang:1.24-alpine AS builder

ARG APP_TARGET=biz-server

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/${APP_TARGET}

FROM alpine:3.20

ARG APP_TARGET=biz-server

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/app /app/${APP_TARGET}

EXPOSE 8080 18080

ENTRYPOINT ["/bin/sh", "-lc", "exec /app/${APP_TARGET} \"$@\"", "--"]
