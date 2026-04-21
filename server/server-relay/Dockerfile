FROM rust:1.87-bookworm AS builder

WORKDIR /src

COPY Cargo.toml Cargo.lock ./
COPY crates ./crates

RUN cargo build -p relay-daemon --bin server-relay --release

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/target/release/server-relay /app/server-relay

EXPOSE 9000/udp

ENTRYPOINT ["/app/server-relay"]
