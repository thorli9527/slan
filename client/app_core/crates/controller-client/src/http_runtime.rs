use serde::{Deserialize, Serialize};

use crate::transport::{HttpMethod, HttpRequest, JsonHttpTransport};

pub fn post_json<T, Req, Resp>(
    transport: &T,
    base_url: &str,
    path: &str,
    bearer_token: Option<&str>,
    body: &Req,
) -> Result<Resp, String>
where
    T: JsonHttpTransport,
    Req: Serialize,
    Resp: for<'de> Deserialize<'de>,
{
    let request = HttpRequest {
        method: HttpMethod::Post,
        path: join_path(base_url, path),
        bearer_token: bearer_token.map(ToOwned::to_owned),
        body_json: Some(serde_json::to_vec(body).map_err(|err| err.to_string())?),
    };
    let response = transport.send(request)?;
    parse_success_response(response.status, &response.body_json)
}

pub fn put_json<T, Req, Resp>(
    transport: &T,
    base_url: &str,
    path: &str,
    bearer_token: Option<&str>,
    body: &Req,
) -> Result<Resp, String>
where
    T: JsonHttpTransport,
    Req: Serialize,
    Resp: for<'de> Deserialize<'de>,
{
    let request = HttpRequest {
        method: HttpMethod::Put,
        path: join_path(base_url, path),
        bearer_token: bearer_token.map(ToOwned::to_owned),
        body_json: Some(serde_json::to_vec(body).map_err(|err| err.to_string())?),
    };
    let response = transport.send(request)?;
    parse_success_response(response.status, &response.body_json)
}

pub fn get_json<T, Resp>(
    transport: &T,
    base_url: &str,
    path: &str,
    bearer_token: Option<&str>,
) -> Result<Resp, String>
where
    T: JsonHttpTransport,
    Resp: for<'de> Deserialize<'de>,
{
    let request = HttpRequest {
        method: HttpMethod::Get,
        path: join_path(base_url, path),
        bearer_token: bearer_token.map(ToOwned::to_owned),
        body_json: None,
    };
    let response = transport.send(request)?;
    parse_success_response(response.status, &response.body_json)
}

fn join_path(base_url: &str, path: &str) -> String {
    format!("{}{}", base_url.trim_end_matches('/'), path)
}

fn parse_success_response<Resp>(status: u16, body_json: &[u8]) -> Result<Resp, String>
where
    Resp: for<'de> Deserialize<'de>,
{
    if !(200..300).contains(&status) {
        return Err(format!("unexpected http status: {status}"));
    }
    serde_json::from_slice(body_json).map_err(|err| err.to_string())
}
