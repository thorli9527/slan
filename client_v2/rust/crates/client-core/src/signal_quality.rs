use serde::{Deserialize, Serialize};

/// 客户端根据当前路径、RTT 与丢包计算出的信号质量快照。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SignalQualityAssessment {
    pub score: u8,
    pub quality: String,
}

pub fn assess_signal_quality(
    network_enabled: bool,
    path: Option<&str>,
    observed_rtt_ms: Option<u32>,
    packet_loss_ppm: Option<u32>,
) -> SignalQualityAssessment {
    if !network_enabled {
        return SignalQualityAssessment {
            score: 0,
            quality: "offline".to_string(),
        };
    }
    let path = path.unwrap_or("unknown").trim().to_ascii_lowercase();
    let mut score: u32 = if path.contains("lan") {
        96
    } else if path.contains("direct") || path == "udp" {
        86
    } else if path.contains("relay_udp") || path.contains("relayudp") {
        68
    } else if path.contains("derp") || path.contains("relay") {
        55
    } else {
        45
    };
    if let Some(loss) = packet_loss_ppm {
        score = score.saturating_sub((loss / 5_000).min(35));
    }
    if let Some(rtt) = observed_rtt_ms {
        score = score.saturating_sub(rtt.saturating_sub(40) / 12);
    }
    let score = score.min(100) as u8;
    SignalQualityAssessment {
        score,
        quality: match score {
            85..=100 => "excellent",
            70..=84 => "good",
            50..=69 => "fair",
            _ => "poor",
        }
        .to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn quality_combines_path_rtt_and_loss() {
        let direct = assess_signal_quality(true, Some("direct_udp"), Some(25), Some(0));
        let degraded = assess_signal_quality(true, Some("relay_udp"), Some(220), Some(50_000));
        assert_eq!(direct.quality, "excellent");
        assert!(direct.score > degraded.score);
        assert_eq!(degraded.quality, "poor");
    }
}
