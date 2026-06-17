use client_core::{PlatformAclPolicy, PlatformAclRule};

use crate::control_plane::{DeviceNetworkConfig, DeviceSecurityRule};

pub(crate) fn platform_acl_policies(configs: &[DeviceNetworkConfig]) -> Vec<PlatformAclPolicy> {
    let mut policies = configs
        .iter()
        .map(|config| {
            (
                config.network_created_at.unwrap_or_default(),
                config.network_id.clone(),
                PlatformAclPolicy {
                    network_id: config.network_id.clone(),
                    rules: platform_acl_rules(config),
                },
            )
        })
        .collect::<Vec<_>>();
    policies.sort_by(|left, right| right.0.cmp(&left.0).then_with(|| right.1.cmp(&left.1)));
    policies.into_iter().map(|(_, _, policy)| policy).collect()
}

pub(crate) fn acl_policies_for_network(
    acl_policies: &[PlatformAclPolicy],
    network_id: &str,
) -> Vec<PlatformAclPolicy> {
    acl_policies
        .iter()
        .filter(|policy| policy.network_id == network_id)
        .cloned()
        .collect()
}

fn platform_acl_rules(config: &DeviceNetworkConfig) -> Vec<PlatformAclRule> {
    let mut rules = config
        .rules
        .iter()
        .map(|rule| {
            let (resolved_peer_node_id, resolved_peer_virtual_ips) =
                resolve_acl_rule_peer(config, rule);
            PlatformAclRule {
                rule_id: rule.rule_id.clone(),
                security_group_id: rule.security_group_id.clone(),
                direction: rule.direction.clone(),
                priority: rule.priority,
                action: rule.action.clone(),
                protocol: rule.protocol.clone(),
                port_from: rule.port_from,
                port_to: rule.port_to,
                peer_type: rule.peer_type.clone(),
                peer_value: rule.peer_value.clone(),
                source_type: rule.peer_type.clone(),
                source_value: rule.peer_value.clone(),
                enabled: rule.enabled,
                resolved_peer_node_id,
                resolved_peer_virtual_ips,
            }
        })
        .collect::<Vec<_>>();
    rules.sort_by(|left, right| {
        left.priority
            .cmp(&right.priority)
            .then_with(|| left.rule_id.cmp(&right.rule_id))
    });
    rules
}

fn resolve_acl_rule_peer(
    config: &DeviceNetworkConfig,
    rule: &DeviceSecurityRule,
) -> (Option<String>, Vec<String>) {
    match rule.peer_type.trim().to_ascii_lowercase().as_str() {
        "device" => {
            let device_id = rule.peer_value.trim();
            let node_id = (!device_id.is_empty()).then(|| format!("node-{device_id}"));
            let mut ips = Vec::new();
            if config.device_id == device_id {
                if let Some(ip) = config.global_ip.clone() {
                    ips.push(ip);
                }
            }
            ips.extend(
                config
                    .peers
                    .iter()
                    .filter(|peer| peer.device_id == device_id)
                    .filter_map(|peer| peer.global_ip.clone()),
            );
            (node_id, ips)
        }
        "domain" => {
            let value = rule.peer_value.trim();
            let mut ips = Vec::new();
            if config
                .global_name
                .as_deref()
                .is_some_and(|name| name.eq_ignore_ascii_case(value))
            {
                if let Some(ip) = config.global_ip.clone() {
                    ips.push(ip);
                }
            }
            ips.extend(
                config
                    .peers
                    .iter()
                    .filter(|peer| {
                        peer.global_name
                            .as_deref()
                            .is_some_and(|name| name.eq_ignore_ascii_case(value))
                            || peer
                                .alias
                                .as_deref()
                                .is_some_and(|alias| alias.eq_ignore_ascii_case(value))
                    })
                    .filter_map(|peer| peer.global_ip.clone()),
            );
            (None, ips)
        }
        _ => (None, Vec::new()),
    }
}

#[cfg(test)]
mod tests {
    use super::{acl_policies_for_network, platform_acl_policies};
    use crate::control_plane::{
        DeviceNetworkConfig, DeviceNetworkPeer, DeviceSecurityGroup, DeviceSecurityRule,
    };

    #[test]
    fn maps_network_acl_rules_in_priority_order_with_resolved_device_peer() {
        let policies = platform_acl_policies(&[DeviceNetworkConfig {
            network_id: "network-1".to_string(),
            device_id: "local-device".to_string(),
            global_ip: Some("10.0.0.2/32".to_string()),
            security_groups: vec![DeviceSecurityGroup {
                security_group_id: "sg-1".to_string(),
                network_id: "network-1".to_string(),
                name: "default".to_string(),
                status: "active".to_string(),
                created_at: 0,
            }],
            peers: vec![DeviceNetworkPeer {
                device_id: "peer-device".to_string(),
                global_ip: Some("10.0.0.9/32".to_string()),
                ..DeviceNetworkPeer::default()
            }],
            rules: vec![
                DeviceSecurityRule {
                    rule_id: "rule-5".to_string(),
                    security_group_id: "sg-1".to_string(),
                    direction: "ingress".to_string(),
                    priority: 5,
                    action: "deny".to_string(),
                    protocol: "all".to_string(),
                    peer_type: "device".to_string(),
                    peer_value: "local-device".to_string(),
                    enabled: true,
                    ..DeviceSecurityRule::default()
                },
                DeviceSecurityRule {
                    rule_id: "rule-20".to_string(),
                    security_group_id: "sg-1".to_string(),
                    direction: "egress".to_string(),
                    priority: 20,
                    action: "deny".to_string(),
                    protocol: "tcp".to_string(),
                    port_from: 443,
                    port_to: 443,
                    peer_type: "device".to_string(),
                    peer_value: "peer-device".to_string(),
                    enabled: true,
                },
                DeviceSecurityRule {
                    rule_id: "rule-10".to_string(),
                    security_group_id: "sg-1".to_string(),
                    direction: "egress".to_string(),
                    priority: 10,
                    action: "allow".to_string(),
                    protocol: "icmp".to_string(),
                    peer_type: "cidr".to_string(),
                    peer_value: "10.0.0.0/24".to_string(),
                    enabled: true,
                    ..DeviceSecurityRule::default()
                },
            ],
            ..DeviceNetworkConfig::default()
        }]);

        assert_eq!(policies.len(), 1);
        assert_eq!(policies[0].rules[0].rule_id, "rule-5");
        assert_eq!(policies[0].rules[1].rule_id, "rule-10");
        assert_eq!(policies[0].rules[2].rule_id, "rule-20");
        assert_eq!(policies[0].rules[0].source_type.as_str(), "device");
        assert_eq!(policies[0].rules[0].source_value.as_str(), "local-device");
        assert_eq!(
            policies[0].rules[0].resolved_peer_node_id.as_deref(),
            Some("node-local-device")
        );
        assert_eq!(
            policies[0].rules[0].resolved_peer_virtual_ips,
            vec!["10.0.0.2/32".to_string()]
        );
        assert_eq!(
            policies[0].rules[2].resolved_peer_node_id.as_deref(),
            Some("node-peer-device")
        );
        assert_eq!(
            policies[0].rules[2].resolved_peer_virtual_ips,
            vec!["10.0.0.9/32".to_string()]
        );
    }

    #[test]
    fn resolves_domain_acl_rule_to_peer_virtual_ip() {
        let policies = platform_acl_policies(&[DeviceNetworkConfig {
            network_id: "network-1".to_string(),
            peers: vec![DeviceNetworkPeer {
                device_id: "peer-device".to_string(),
                alias: Some("build-host".to_string()),
                global_name: Some("build.slan".to_string()),
                global_ip: Some("10.0.0.7/32".to_string()),
                ..DeviceNetworkPeer::default()
            }],
            rules: vec![DeviceSecurityRule {
                rule_id: "rule-domain".to_string(),
                security_group_id: "sg-1".to_string(),
                direction: "egress".to_string(),
                priority: 1,
                action: "deny".to_string(),
                protocol: "all".to_string(),
                peer_type: "domain".to_string(),
                peer_value: "BUILD.SLAN".to_string(),
                enabled: true,
                ..DeviceSecurityRule::default()
            }],
            ..DeviceNetworkConfig::default()
        }]);

        assert_eq!(
            policies[0].rules[0].resolved_peer_virtual_ips,
            vec!["10.0.0.7/32".to_string()]
        );
    }

    #[test]
    fn filters_acl_policies_by_active_network() {
        let policies = platform_acl_policies(&[
            DeviceNetworkConfig {
                network_id: "network-a".to_string(),
                ..DeviceNetworkConfig::default()
            },
            DeviceNetworkConfig {
                network_id: "network-b".to_string(),
                ..DeviceNetworkConfig::default()
            },
        ]);

        let filtered = acl_policies_for_network(&policies, "network-b");

        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].network_id, "network-b");
    }

    #[test]
    fn orders_acl_policies_by_newer_network_first_for_override() {
        let policies = platform_acl_policies(&[
            DeviceNetworkConfig {
                network_id: "old-network".to_string(),
                network_created_at: Some(10),
                rules: vec![DeviceSecurityRule {
                    rule_id: "old-allow".to_string(),
                    direction: "egress".to_string(),
                    priority: 1,
                    action: "allow".to_string(),
                    protocol: "tcp".to_string(),
                    port_from: 443,
                    port_to: 443,
                    peer_type: "all".to_string(),
                    peer_value: "all".to_string(),
                    enabled: true,
                    ..DeviceSecurityRule::default()
                }],
                ..DeviceNetworkConfig::default()
            },
            DeviceNetworkConfig {
                network_id: "new-network".to_string(),
                network_created_at: Some(20),
                rules: vec![DeviceSecurityRule {
                    rule_id: "new-deny".to_string(),
                    direction: "egress".to_string(),
                    priority: 1,
                    action: "deny".to_string(),
                    protocol: "tcp".to_string(),
                    port_from: 443,
                    port_to: 443,
                    peer_type: "all".to_string(),
                    peer_value: "all".to_string(),
                    enabled: true,
                    ..DeviceSecurityRule::default()
                }],
                ..DeviceNetworkConfig::default()
            },
        ]);

        assert_eq!(policies[0].network_id, "new-network");
        assert_eq!(policies[0].rules[0].rule_id, "new-deny");
        assert_eq!(policies[1].network_id, "old-network");
    }
}
