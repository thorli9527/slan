FROM golang:1.26.3-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/service-biz-new ./cmd/service-biz-new

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/service-biz-new /app/service-biz-new

EXPOSE 8080

ENTRYPOINT ["/app/service-biz-new"]
