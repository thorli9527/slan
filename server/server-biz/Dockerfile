FROM golang:1.25-alpine AS builder

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

COPY --from=builder /out/app /app/app

EXPOSE 8080 8081 3478/udp

ENTRYPOINT ["/app/app"]
