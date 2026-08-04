#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) struct TicketTiming {
    pub(crate) expires_in_ms: Option<i64>,
    pub(crate) renew_due: bool,
}

pub(crate) fn ticket_timing_with_window(
    now_ms: u64,
    ticket_expires_at: Option<&str>,
    renew_window_ms: u64,
) -> TicketTiming {
    let Some(expires_at_ms) = ticket_expires_at.and_then(parse_rfc3339_utc_ms) else {
        return TicketTiming {
            expires_in_ms: None,
            renew_due: false,
        };
    };
    let expires_in_ms = expires_at_ms as i128 - now_ms as i128;
    TicketTiming {
        expires_in_ms: Some(expires_in_ms.clamp(i64::MIN as i128, i64::MAX as i128) as i64),
        renew_due: now_ms.saturating_add(renew_window_ms) >= expires_at_ms,
    }
}

pub(crate) fn parse_rfc3339_utc_ms(value: &str) -> Option<u64> {
    let value = value.trim();
    let (date, time) = value.split_once('T')?;
    let time = time.strip_suffix('Z')?;
    let mut date_parts = date.split('-');
    let year = date_parts.next()?.parse::<i32>().ok()?;
    let month = date_parts.next()?.parse::<u32>().ok()?;
    let day = date_parts.next()?.parse::<u32>().ok()?;
    if date_parts.next().is_some() {
        return None;
    }
    let time = time.split_once('.').map(|(whole, _)| whole).unwrap_or(time);
    let mut time_parts = time.split(':');
    let hour = time_parts.next()?.parse::<u32>().ok()?;
    let minute = time_parts.next()?.parse::<u32>().ok()?;
    let second = time_parts.next()?.parse::<u32>().ok()?;
    if time_parts.next().is_some()
        || !(1..=12).contains(&month)
        || !(1..=31).contains(&day)
        || hour > 23
        || minute > 59
        || second > 59
    {
        return None;
    }
    let days = days_from_civil(year, month, day)?;
    let seconds = days
        .checked_mul(86_400)?
        .checked_add(i64::from(hour) * 3_600)?
        .checked_add(i64::from(minute) * 60)?
        .checked_add(i64::from(second))?;
    u64::try_from(seconds).ok()?.checked_mul(1_000)
}

fn days_from_civil(year: i32, month: u32, day: u32) -> Option<i64> {
    let mut year = i64::from(year);
    let month = i64::from(month);
    let day = i64::from(day);
    year -= if month <= 2 { 1 } else { 0 };
    let era = if year >= 0 { year } else { year - 399 } / 400;
    let yoe = year - era * 400;
    let month_prime = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * month_prime + 2) / 5 + day - 1;
    if !(0..=365).contains(&doy) {
        return None;
    }
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    Some(era * 146_097 + doe - 719_468)
}
