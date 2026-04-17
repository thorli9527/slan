use crate::RelayError;

/// 解析 unix 时间戳或 RFC3339 时间字符串。
pub fn parse_timestamp(raw: &str) -> Result<i64, RelayError> {
    let raw = raw.trim();
    if raw.is_empty() {
        return Err(RelayError::InvalidTimestamp);
    }

    if let Ok(seconds) = raw.parse::<i64>() {
        return Ok(seconds);
    }

    parse_rfc3339(raw).ok_or(RelayError::InvalidTimestamp)
}

fn parse_rfc3339(raw: &str) -> Option<i64> {
    if raw.len() < 20 {
        return None;
    }

    let year = raw.get(0..4)?.parse::<i32>().ok()?;
    let month = raw.get(5..7)?.parse::<u32>().ok()?;
    let day = raw.get(8..10)?.parse::<u32>().ok()?;
    let hour = raw.get(11..13)?.parse::<u32>().ok()?;
    let minute = raw.get(14..16)?.parse::<u32>().ok()?;
    let second = raw.get(17..19)?.parse::<u32>().ok()?;

    let rest = raw.get(19..)?;
    let offset_seconds = match rest.as_bytes().first().copied()? as char {
        'Z' => 0,
        '+' | '-' => {
            if rest.len() != 6 {
                return None;
            }
            let sign = if &rest[0..1] == "+" { 1 } else { -1 };
            let offset_hour = rest.get(1..3)?.parse::<i64>().ok()?;
            let offset_minute = rest.get(4..6)?.parse::<i64>().ok()?;
            sign * (offset_hour * 3600 + offset_minute * 60)
        }
        _ => return None,
    };

    let days = days_from_civil(year, month, day)?;
    let seconds =
        days * 86_400 + i64::from(hour) * 3600 + i64::from(minute) * 60 + i64::from(second);
    Some(seconds - offset_seconds)
}

fn days_from_civil(year: i32, month: u32, day: u32) -> Option<i64> {
    if !(1..=12).contains(&month) || !(1..=31).contains(&day) {
        return None;
    }

    let year = year - i32::from(month <= 2);
    let era = if year >= 0 { year } else { year - 399 } / 400;
    let yoe = year - era * 400;
    let month = month as i32;
    let doy = (153 * (month + if month > 2 { -3 } else { 9 }) + 2) / 5 + day as i32 - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    Some(i64::from(era * 146097 + doe - 719468))
}
