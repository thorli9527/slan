# GeoIP data

Place `GeoLite2-City.mmdb` in this directory to enable country/city-aware
infrastructure node selection. The database is mounted read-only into
`service-biz`; it is intentionally not committed or built into the image.

When the file is absent, runtime selection falls back to transport, health,
priority, and measured RTT without blocking client requests.

When `service-biz` is behind a reverse proxy, configure
`SLAN_TRUSTED_PROXY_CIDRS` with only that proxy network. Client geolocation uses
the existing trusted-proxy-aware remote IP parser and does not trust forwarded
headers from arbitrary peers.
