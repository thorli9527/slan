use std::io::{Read, Write};
use std::net::{Shutdown, TcpStream, ToSocketAddrs};
use std::time::Duration;

#[derive(Debug, Clone, Copy)]
pub enum HttpMethod {
    Get,
    Post,
    Put,
}

#[derive(Debug, Clone)]
pub struct HttpRequest {
    pub method: HttpMethod,
    pub path: String,
    pub bearer_token: Option<String>,
    pub body_json: Option<Vec<u8>>,
}

#[derive(Debug, Clone)]
pub struct HttpResponse {
    pub status: u16,
    pub body_json: Vec<u8>,
}

pub trait JsonHttpTransport: Send + Sync {
    fn send(&self, request: HttpRequest) -> Result<HttpResponse, String>;
}

#[derive(Debug, Clone)]
pub struct TcpJsonHttpTransport {
    timeout: Duration,
}

impl Default for TcpJsonHttpTransport {
    fn default() -> Self {
        Self {
            timeout: Duration::from_secs(5),
        }
    }
}

impl TcpJsonHttpTransport {
    pub fn new(timeout: Duration) -> Self {
        Self { timeout }
    }
}

impl JsonHttpTransport for TcpJsonHttpTransport {
    fn send(&self, request: HttpRequest) -> Result<HttpResponse, String> {
        let target = ParsedHttpUrl::parse(&request.path)?;
        let address = format!("{}:{}", target.host, target.port);
        let socket_addr = address
            .to_socket_addrs()
            .map_err(|err| err.to_string())?
            .next()
            .ok_or_else(|| format!("unable to resolve host: {}", target.host))?;
        let mut stream = TcpStream::connect_timeout(&socket_addr, self.timeout)
            .map_err(|err| err.to_string())?;
        stream
            .set_read_timeout(Some(self.timeout))
            .map_err(|err| err.to_string())?;
        stream
            .set_write_timeout(Some(self.timeout))
            .map_err(|err| err.to_string())?;

        let body = request.body_json.unwrap_or_default();
        let method = match request.method {
            HttpMethod::Get => "GET",
            HttpMethod::Post => "POST",
            HttpMethod::Put => "PUT",
        };
        let mut wire = format!(
            "{method} {} HTTP/1.1\r\nHost: {}\r\nAccept: application/json\r\nConnection: close\r\n",
            target.path_and_query, target.host_header
        );
        if let Some(token) = request.bearer_token {
            wire.push_str(&format!("Authorization: Bearer {token}\r\n"));
        }
        if !body.is_empty() {
            wire.push_str("Content-Type: application/json\r\n");
            wire.push_str(&format!("Content-Length: {}\r\n", body.len()));
        }
        wire.push_str("\r\n");

        stream
            .write_all(wire.as_bytes())
            .and_then(|_| {
                if body.is_empty() {
                    Ok(())
                } else {
                    stream.write_all(&body)
                }
            })
            .map_err(|err| err.to_string())?;
        let _ = stream.shutdown(Shutdown::Write);

        let mut raw = Vec::new();
        stream
            .read_to_end(&mut raw)
            .map_err(|err| err.to_string())?;
        parse_http_response(&raw)
    }
}

#[derive(Debug, Clone)]
struct ParsedHttpUrl {
    host: String,
    host_header: String,
    port: u16,
    path_and_query: String,
}

impl ParsedHttpUrl {
    fn parse(raw: &str) -> Result<Self, String> {
        let raw = raw.trim();
        let rest = raw
            .strip_prefix("http://")
            .ok_or_else(|| format!("unsupported url scheme: {raw}"))?;
        let (host_port, path_and_query) = match rest.split_once('/') {
            Some((host_port, path)) => (host_port, format!("/{}", path)),
            None => (rest, "/".to_string()),
        };
        let (host, port) = match host_port.split_once(':') {
            Some((host, port)) => (
                host.to_string(),
                port.parse::<u16>()
                    .map_err(|err| format!("invalid port in url {raw}: {err}"))?,
            ),
            None => (host_port.to_string(), 80),
        };
        if host.is_empty() {
            return Err(format!("missing host in url: {raw}"));
        }
        Ok(Self {
            host_header: if port == 80 {
                host.clone()
            } else {
                format!("{host}:{port}")
            },
            host,
            port,
            path_and_query,
        })
    }
}

fn parse_http_response(raw: &[u8]) -> Result<HttpResponse, String> {
    let separator = b"\r\n\r\n";
    let header_end = raw
        .windows(separator.len())
        .position(|window| window == separator)
        .ok_or_else(|| "invalid http response: missing header separator".to_string())?;
    let header_bytes = &raw[..header_end];
    let body = raw[header_end + separator.len()..].to_vec();
    let header_text = String::from_utf8(header_bytes.to_vec()).map_err(|err| err.to_string())?;
    let mut lines = header_text.lines();
    let status_line = lines
        .next()
        .ok_or_else(|| "invalid http response: missing status line".to_string())?;
    let status = status_line
        .split_whitespace()
        .nth(1)
        .ok_or_else(|| format!("invalid http status line: {status_line}"))?
        .parse::<u16>()
        .map_err(|err| err.to_string())?;
    let chunked = lines.any(|line| {
        let line = line.trim();
        line.eq_ignore_ascii_case("transfer-encoding: chunked")
            || line
                .to_ascii_lowercase()
                .starts_with("transfer-encoding: chunked")
    });
    let body_json = if chunked {
        decode_chunked_body(&body)?
    } else {
        body
    };
    Ok(HttpResponse { status, body_json })
}

fn decode_chunked_body(body: &[u8]) -> Result<Vec<u8>, String> {
    let mut offset = 0usize;
    let mut decoded = Vec::new();
    loop {
        let line_end = find_crlf(body, offset)
            .ok_or_else(|| "invalid chunked response: missing chunk size".to_string())?;
        let size_line =
            std::str::from_utf8(&body[offset..line_end]).map_err(|err| err.to_string())?;
        let size_text = size_line.split(';').next().unwrap_or("").trim();
        let size = usize::from_str_radix(size_text, 16)
            .map_err(|err| format!("invalid chunked response size {size_text:?}: {err}"))?;
        offset = line_end + 2;
        if size == 0 {
            return Ok(decoded);
        }
        let chunk_end = offset
            .checked_add(size)
            .ok_or_else(|| "invalid chunked response: chunk size overflow".to_string())?;
        if chunk_end + 2 > body.len() {
            return Err("invalid chunked response: truncated chunk".to_string());
        }
        decoded.extend_from_slice(&body[offset..chunk_end]);
        if &body[chunk_end..chunk_end + 2] != b"\r\n" {
            return Err("invalid chunked response: missing chunk terminator".to_string());
        }
        offset = chunk_end + 2;
    }
}

fn find_crlf(bytes: &[u8], start: usize) -> Option<usize> {
    bytes
        .get(start..)?
        .windows(2)
        .position(|window| window == b"\r\n")
        .map(|position| start + position)
}

#[cfg(test)]
mod tests {
    use super::parse_http_response;

    #[test]
    fn parse_http_response_decodes_chunked_body() {
        let raw =
            b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n8\r\n{\"ok\":1}\r\n0\r\n\r\n";
        let response = parse_http_response(raw).expect("response");

        assert_eq!(response.status, 200);
        assert_eq!(response.body_json, br#"{"ok":1}"#);
    }
}
