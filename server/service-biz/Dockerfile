FROM golang:1.26.3-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o /out/service-biz ./cmd/service-biz

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

EXPOSE 8080

ENTRYPOINT ["/app/service-biz"]
