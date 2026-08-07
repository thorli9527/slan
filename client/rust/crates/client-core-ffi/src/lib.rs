use std::{
    ffi::{c_char, CStr, CString},
    ptr,
};

use client_core::ClientViewState;

#[cfg(target_os = "android")]
mod android_tun {
    use std::{
        ffi::CString,
        fs::File,
        io::{BufRead, BufReader, ErrorKind, Read, Write},
        net::{TcpStream, UdpSocket},
        os::fd::FromRawFd,
        os::raw::c_int,
        sync::{
            atomic::{AtomicBool, AtomicU64, Ordering},
            Arc, Mutex, OnceLock,
        },
        thread::{self, JoinHandle},
        time::{Duration, Instant, SystemTime, UNIX_EPOCH},
    };

    use client_core::relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    };
    use client_core::{
        acl_allows_egress_packet, acl_allows_ingress_packet, icmp_echo_reply_for_request,
        ipv4_transport_checksum_valid, normalize_ipv4_transport_checksums,
        resolver_response_for_query, AndroidVpnSessionConfig, PlatformAclPeer,
        PlatformResolverRecord, RelayPeerSession,
    };
    use client_core_platform::direct_udp::{
        direct_udp_control_packet, DirectUdpControlKind, DirectUdpTransport,
    };
    use jni::{
        objects::{JClass, JIntArray, JString},
        JNIEnv,
    };
    use libc::c_char;

    // Android relay UDP mappings on some networks/emulators appear to expire
    // within ~10-15s, which can drop server->client packet delivery while
    // control traffic still looks healthy. Keep the relay pinhole warm more
    // aggressively.
    const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(5);
    const DERP_WRITE_RETRY_TIMEOUT: Duration = Duration::from_millis(750);
    const DATA_PLANE_IDLE_SLEEP: Duration = Duration::from_millis(2);
    const ANDROID_LOG_TAG: &[u8] = b"client-core-ffi\0";
    const ANDROID_LOG_INFO: i32 = 4;
    const ANDROID_LOG_ERROR: i32 = 6;

    fn current_timestamp_ms() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis() as u64)
            .unwrap_or_default()
    }

    unsafe extern "C" {
        fn __android_log_write(prio: i32, tag: *const c_char, text: *const c_char) -> i32;
    }

    fn android_log(priority: i32, message: &str) {
        let sanitized = message.replace('\0', "\\0");
        if let Ok(text) = CString::new(sanitized) {
            unsafe {
                __android_log_write(
                    priority,
                    ANDROID_LOG_TAG.as_ptr().cast::<c_char>(),
                    text.as_ptr(),
                );
            }
        }
    }

    fn android_log_info(message: &str) {
        android_log(ANDROID_LOG_INFO, message);
    }

    fn android_log_error(message: &str) {
        android_log(ANDROID_LOG_ERROR, message);
    }

    /// TunRuntime 持有 Android TUN fd 数据面线程、运行统计和原始配置。
    struct TunRuntime {
        stop: Arc<AtomicBool>,
        stats: Arc<TunStats>,
        last_attach_error: Arc<Mutex<Option<String>>>,
        relay_peer_virtual_ips: Arc<Mutex<Vec<String>>>,
        derp_peer_virtual_ips: Arc<Mutex<Vec<String>>>,
        relay_session_ids: Arc<Mutex<Vec<String>>>,
        handle: Option<JoinHandle<()>>,
        config_json: String,
    }

    /// TunStats 是 Android 原生层返回给 Flutter 的 TUN/relay/direct UDP 统计。
    #[derive(Default)]
    struct TunStats {
        requested_relay_session_count: AtomicU64,
        attached_relay_session_count: AtomicU64,
        relay_attach_failures: AtomicU64,
        packets_read: AtomicU64,
        bytes_read: AtomicU64,
        bytes_written: AtomicU64,
        packets_too_large: AtomicU64,
        relay_frames_sent: AtomicU64,
        relay_frames_received: AtomicU64,
        relay_control_packets_received: AtomicU64,
        relay_decode_failures: AtomicU64,
        relay_tcp_frames_received: AtomicU64,
        relay_tcp_syn_ack_received: AtomicU64,
        relay_tcp_psh_received: AtomicU64,
        relay_tcp_rst_received: AtomicU64,
        relay_tcp_checksum_invalid: AtomicU64,
        relay_detach_sent: AtomicU64,
        relay_no_peer_packets: AtomicU64,
        direct_udp_attached_peer_count: AtomicU64,
        direct_udp_ready_peer_count: AtomicU64,
        direct_udp_probes_sent: AtomicU64,
        direct_udp_probes_received: AtomicU64,
        direct_udp_pongs_sent: AtomicU64,
        direct_udp_pongs_received: AtomicU64,
        direct_udp_frames_sent: AtomicU64,
        direct_udp_frames_received: AtomicU64,
        last_no_peer_destination: Mutex<Option<String>>,
        last_no_peer_packet: Mutex<Option<String>>,
        relay_write_failures: AtomicU64,
        tun_write_failures: AtomicU64,
        last_relay_control_kind: Mutex<Option<String>>,
        last_relay_raw_frame: Mutex<Option<String>>,
        last_tun_write_error: Mutex<Option<String>>,
        last_tun_write_packet: Mutex<Option<String>>,
    }

    impl Drop for TunRuntime {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::SeqCst);
            if let Some(handle) = self.handle.take() {
                let _ = handle.join();
            }
        }
    }

    static TUN_RUNTIME: OnceLock<Mutex<Option<TunRuntime>>> = OnceLock::new();

    fn runtime() -> &'static Mutex<Option<TunRuntime>> {
        TUN_RUNTIME.get_or_init(|| Mutex::new(None))
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_startTun(
        mut env: JNIEnv,
        _class: JClass,
        tun_fd: c_int,
        relay_fds: JIntArray,
        config_json: JString,
    ) -> c_int {
        if tun_fd < 0 {
            return -1;
        }
        let config = env
            .get_string(&config_json)
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_default();
        let relay_fds = read_relay_fds(&mut env, relay_fds).unwrap_or_default();
        match start_tun(tun_fd, relay_fds, config) {
            Ok(()) => 0,
            Err(_) => -2,
        }
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_stopTun(
        _env: JNIEnv,
        _class: JClass,
    ) {
        stop_tun();
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_isTunRunning(
        _env: JNIEnv,
        _class: JClass,
    ) -> c_int {
        let guard = runtime()
            .lock()
            .expect("android tun runtime mutex poisoned");
        if guard.is_some() {
            1
        } else {
            0
        }
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_tunStatsJson<'a>(
        env: JNIEnv<'a>,
        _class: JClass<'a>,
    ) -> JString<'a> {
        let json = {
            let guard = runtime()
                .lock()
                .expect("android tun runtime mutex poisoned");
            guard
                .as_ref()
                .map(tun_stats_json)
                .unwrap_or_else(|| "{}".to_string())
        };
        env.new_string(json)
            .unwrap_or_else(|_| env.new_string("{}").expect("static json string"))
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_serviceRequestJson<
        'a,
    >(
        mut env: JNIEnv<'a>,
        _class: JClass<'a>,
        request_json: JString<'a>,
    ) -> JString<'a> {
        let request = env
            .get_string(&request_json)
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_default();
        let response = client_core_service::embedded::embedded_handle_request_json(&request);
        env.new_string(response).unwrap_or_else(|_| {
            env.new_string("{\"error\":\"encode response\"}")
                .expect("static json string")
        })
    }

    #[no_mangle]
    pub extern "system" fn Java_dev_slan_client_1core_1plugin_SlanNativeBridge_initializeRuntime<
        'a,
    >(
        mut env: JNIEnv<'a>,
        _class: JClass<'a>,
        state_dir: JString<'a>,
    ) -> JString<'a> {
        let state_dir = env
            .get_string(&state_dir)
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_default();
        let response = super::embedded_initialize_json(&state_dir);
        env.new_string(response).unwrap_or_else(|_| {
            env.new_string("{\"error\":\"encode initialization response\"}")
                .expect("static json string")
        })
    }

    /// AndroidRelayPeer 是 Rust 数据面中一个 relay peer 会话及其受保护 socket。
    struct AndroidRelayPeer {
        session_id: String,
        peer_node_id: String,
        local_node_id: String,
        peer_virtual_ips: Vec<String>,
        socket: UdpSocket,
    }

    struct AndroidDerpPeer {
        peer_node_id: String,
        local_node_id: String,
        server_session_id: String,
        peer_virtual_ips: Vec<String>,
        stream: TcpStream,
        reader: TcpStream,
        read_buffer: Vec<u8>,
    }

    /// 启动 Android TUN 数据面，接管 Java 层 detach 出来的 TUN fd 和 UDP socket fd。
    fn start_tun(tun_fd: c_int, relay_fds: Vec<c_int>, config_json: String) -> std::io::Result<()> {
        stop_tun();
        set_nonblocking(tun_fd);
        let parsed_config = serde_json::from_str::<AndroidVpnSessionConfig>(&config_json).ok();
        let native_resolver_records = serde_json::from_str::<serde_json::Value>(&config_json)
            .ok()
            .and_then(|value| value.pointer("/resolver/records").cloned())
            .and_then(|value| serde_json::from_value::<Vec<PlatformResolverRecord>>(value).ok())
            .unwrap_or_default();
        let stats = Arc::new(TunStats::default());
        let last_attach_error = Arc::new(Mutex::new(None));
        stats.requested_relay_session_count.store(
            requested_relay_session_count(parsed_config.as_ref()),
            Ordering::Relaxed,
        );
        let (relay_peers, derp_peers, direct_udp) = prepare_relay_sockets(
            relay_fds,
            parsed_config.as_ref(),
            Arc::clone(&last_attach_error),
        )?;
        let attached_relay_count = relay_peers.len().saturating_add(derp_peers.len());
        let relay_peer_virtual_ips = Arc::new(Mutex::new(
            relay_peers
                .iter()
                .map(|peer| format!("{}:{}", peer.peer_node_id, peer.peer_virtual_ips.join("|")))
                .collect::<Vec<_>>(),
        ));
        let derp_peer_virtual_ips = Arc::new(Mutex::new(
            derp_peers
                .iter()
                .map(|peer| format!("{}:{}", peer.peer_node_id, peer.peer_virtual_ips.join("|")))
                .collect::<Vec<_>>(),
        ));
        let relay_session_ids = Arc::new(Mutex::new(
            relay_peers
                .iter()
                .map(|peer| peer.session_id.clone())
                .chain(derp_peers.iter().map(|peer| peer.server_session_id.clone()))
                .collect::<Vec<_>>(),
        ));
        stats.direct_udp_attached_peer_count.store(
            direct_udp
                .as_ref()
                .map(|transport| transport.peers.len() as u64)
                .unwrap_or(0),
            Ordering::Relaxed,
        );
        stats
            .attached_relay_session_count
            .store(attached_relay_count as u64, Ordering::Relaxed);
        stats.relay_attach_failures.store(
            stats
                .requested_relay_session_count
                .load(Ordering::Relaxed)
                .saturating_sub(attached_relay_count as u64),
            Ordering::Relaxed,
        );
        let max_frame_payload = parsed_config
            .as_ref()
            .and_then(|config| config.relay_data_plane.as_ref())
            .and_then(|config| config.max_frame_payload)
            .unwrap_or(1200)
            .clamp(512, 1400) as usize;
        let local_virtual_ip = parsed_config
            .as_ref()
            .map(|config| config.virtual_ip.clone())
            .unwrap_or_default();
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let thread_stats = Arc::clone(&stats);
        let thread_config_json = config_json.clone();
        let handle = thread::spawn(move || {
            let mut file = unsafe { File::from_raw_fd(tun_fd) };
            let relay_peers = relay_peers;
            let mut derp_peers = derp_peers;
            let config_hash = stable_hash64(&thread_config_json);
            let mut seq = 0_u64;
            let mut tun_buffer = vec![0_u8; 2048];
            let mut relay_buffer = vec![0_u8; 4096];
            let mut direct_udp = direct_udp;
            let direct_udp_probe_interval = Duration::from_secs(1);
            let direct_udp_network_id = parsed_config
                .as_ref()
                .and_then(|config| config.relay_data_plane.as_ref())
                .map(|config| config.network_id.clone())
                .unwrap_or_default();
            let direct_udp_node_configs = parsed_config
                .as_ref()
                .and_then(|config| config.relay_data_plane.as_ref())
                .map(|config| config.node_configs.clone())
                .unwrap_or_default();
            let mut last_direct_udp_probe = Instant::now()
                .checked_sub(direct_udp_probe_interval)
                .unwrap_or_else(Instant::now);
            let mut last_keepalive = Instant::now()
                .checked_sub(RELAY_KEEPALIVE_INTERVAL)
                .unwrap_or_else(Instant::now);
            let acl_policies = parsed_config
                .as_ref()
                .and_then(|config| config.relay_data_plane.as_ref())
                .map(|config| config.acl_policies.clone())
                .unwrap_or_default();
            let resolver_records = parsed_config
                .as_ref()
                .map(|config| config.resolver_records.clone())
                .filter(|records| !records.is_empty())
                .unwrap_or(native_resolver_records);
            let resolver_servers = parsed_config
                .as_ref()
                .map(|config| config.resolver.servers.clone())
                .unwrap_or_default();
            while !thread_stop.load(Ordering::SeqCst) {
                let mut did_work = false;
                if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
                    if let Some(direct_udp) = direct_udp.as_mut() {
                        for peer_node_id in direct_udp.poll_probe_health(current_timestamp_ms()) {
                            android_log_info(&format!(
                                "SLAN_ANDROID_DIRECT_UDP_PEER_FAILED peer={}",
                                peer_node_id
                            ));
                        }
                        thread_stats
                            .direct_udp_ready_peer_count
                            .store(direct_udp.ready_peer_count() as u64, Ordering::Relaxed);
                        let punch_probes = direct_udp.send_punch_endpoint_probes(
                            direct_udp_network_id.as_str(),
                            &direct_udp_node_configs,
                        );
                        thread_stats
                            .direct_udp_probes_sent
                            .fetch_add(direct_udp.send_probe_packets() as u64, Ordering::Relaxed);
                        if punch_probes > 0 {
                            android_log_info(&format!(
                                "SLAN_ANDROID_FFI_PUNCH_PROBES_SENT count={} networkId={} localNodeId={}",
                                punch_probes,
                                direct_udp_network_id,
                                direct_udp.local_node_id,
                            ));
                        }
                    }
                    last_direct_udp_probe = Instant::now();
                }
                if last_keepalive.elapsed() >= RELAY_KEEPALIVE_INTERVAL {
                    send_relay_keepalives(&relay_peers);
                    last_keepalive = Instant::now();
                }
                match file.read(&mut tun_buffer) {
                    Ok(0) => {}
                    Ok(packet_len) => {
                        did_work = true;
                        thread_stats.packets_read.fetch_add(1, Ordering::Relaxed);
                        thread_stats
                            .bytes_read
                            .fetch_add(packet_len as u64, Ordering::Relaxed);
                        let packet = &tun_buffer[..packet_len];
                        if let Some(reply) =
                            local_dns_reply(packet, &resolver_servers, &resolver_records)
                        {
                            write_android_tun_inbound_packet(&mut file, &thread_stats, &reply);
                            continue;
                        }
                        if packet_targets_local_virtual_ip(packet, local_virtual_ip.as_str()) {
                            if let Some(reply) =
                                local_virtual_ip_reply(packet, local_virtual_ip.as_str())
                            {
                                write_android_tun_inbound_packet(&mut file, &thread_stats, &reply);
                            } else {
                                write_android_tun_inbound_packet(&mut file, &thread_stats, packet);
                            }
                            continue;
                        }
                        if packet_len > max_frame_payload {
                            thread_stats
                                .packets_too_large
                                .fetch_add(1, Ordering::Relaxed);
                            continue;
                        }
                        if let Some(direct_udp) = direct_udp.as_ref() {
                            if let Some(peer_index) =
                                direct_udp.ready_peer_index_for_packet(&tun_buffer[..packet_len])
                            {
                                let peer = &direct_udp.peers[peer_index];
                                if !acl_allows_egress_packet(
                                    &tun_buffer[..packet_len],
                                    &acl_policies,
                                    Some(&acl_peer_for_direct_peer(peer)),
                                ) {
                                    continue;
                                }
                                seq = seq.wrapping_add(1);
                                let packet =
                                    normalize_ipv4_transport_checksums(&tun_buffer[..packet_len]);
                                if let Some(frame) =
                                    encode_slan_relay_data_frame(seq, config_hash, &packet)
                                {
                                    let mut sent = false;
                                    for attempt in 0..relay_send_attempt_count(&packet) {
                                        if attempt > 0 {
                                            thread::sleep(relay_send_attempt_delay(&packet));
                                        }
                                        if direct_udp.send_to_peer(peer_index, &frame).is_ok() {
                                            sent = true;
                                            thread_stats
                                                .direct_udp_frames_sent
                                                .fetch_add(1, Ordering::Relaxed);
                                        }
                                    }
                                    if should_hedge_direct_packet_to_relay(&packet) {
                                        sent |= hedge_udp_packet_to_relay(
                                            &relay_peers,
                                            &mut derp_peers,
                                            &packet,
                                            &frame,
                                            &thread_stats,
                                        );
                                    }
                                    if sent {
                                        continue;
                                    }
                                }
                            }
                        }
                        if let Some(peer) =
                            relay_peer_for_packet(&relay_peers, &tun_buffer[..packet_len])
                        {
                            if !acl_allows_egress_packet(
                                &tun_buffer[..packet_len],
                                &acl_policies,
                                Some(&acl_peer_for_relay_peer(peer)),
                            ) {
                                continue;
                            }
                            seq = seq.wrapping_add(1);
                            let packet =
                                normalize_ipv4_transport_checksums(&tun_buffer[..packet_len]);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
                            {
                                let mut relay_sent = false;
                                if let Some(payload) = encode_relay_forward(peer, &frame) {
                                    for attempt in 0..relay_send_attempt_count(&packet) {
                                        if attempt > 0 {
                                            thread::sleep(relay_send_attempt_delay(&packet));
                                        }
                                        android_log_info(&format!(
                                            "SLAN_ANDROID_FFI_RELAY_SEND sessionId={} peerNodeId={} attempt={} payloadBytes={} frameBytes={} packet={}",
                                            peer.session_id,
                                            peer.peer_node_id,
                                            attempt + 1,
                                            payload.len(),
                                            frame.len(),
                                            packet_summary(&packet),
                                        ));
                                        match peer.socket.send(&payload) {
                                            Ok(_) => {
                                                relay_sent = true;
                                                android_log_info(&format!(
                                                    "SLAN_ANDROID_FFI_RELAY_SEND_OK sessionId={} peerNodeId={} attempt={} payloadBytes={}",
                                                    peer.session_id,
                                                    peer.peer_node_id,
                                                    attempt + 1,
                                                    payload.len(),
                                                ));
                                                thread_stats
                                                    .relay_frames_sent
                                                    .fetch_add(1, Ordering::Relaxed);
                                            }
                                            Err(error) => {
                                                android_log_error(&format!(
                                                    "SLAN_ANDROID_FFI_RELAY_SEND_ERROR sessionId={} peerNodeId={} attempt={} payloadBytes={} error={}",
                                                    peer.session_id,
                                                    peer.peer_node_id,
                                                    attempt + 1,
                                                    payload.len(),
                                                    error,
                                                ));
                                                thread_stats
                                                    .relay_write_failures
                                                    .fetch_add(1, Ordering::Relaxed);
                                            }
                                        }
                                    }
                                } else {
                                    thread_stats
                                        .relay_write_failures
                                        .fetch_add(1, Ordering::Relaxed);
                                }
                                if !relay_sent {
                                    if let Some(peer) =
                                        derp_peer_for_packet_mut(&mut derp_peers, &packet)
                                    {
                                        if !acl_allows_egress_packet(
                                            &packet,
                                            &acl_policies,
                                            Some(&acl_peer_for_derp_peer(peer)),
                                        ) {
                                            continue;
                                        }
                                        for attempt in 0..relay_send_attempt_count(&packet) {
                                            if attempt > 0 {
                                                thread::sleep(relay_send_attempt_delay(&packet));
                                            }
                                            match send_derp_forward(peer, &frame) {
                                                Ok(_) => {
                                                    thread_stats
                                                        .relay_frames_sent
                                                        .fetch_add(1, Ordering::Relaxed);
                                                }
                                                Err(_) => {
                                                    thread_stats
                                                        .relay_write_failures
                                                        .fetch_add(1, Ordering::Relaxed);
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        } else if let Some(peer) =
                            derp_peer_for_packet_mut(&mut derp_peers, &tun_buffer[..packet_len])
                        {
                            if !acl_allows_egress_packet(
                                &tun_buffer[..packet_len],
                                &acl_policies,
                                Some(&acl_peer_for_derp_peer(peer)),
                            ) {
                                continue;
                            }
                            seq = seq.wrapping_add(1);
                            let packet =
                                normalize_ipv4_transport_checksums(&tun_buffer[..packet_len]);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
                            {
                                for attempt in 0..relay_send_attempt_count(&packet) {
                                    if attempt > 0 {
                                        thread::sleep(relay_send_attempt_delay(&packet));
                                    }
                                    match send_derp_forward(peer, &frame) {
                                        Ok(_) => {
                                            thread_stats
                                                .relay_frames_sent
                                                .fetch_add(1, Ordering::Relaxed);
                                        }
                                        Err(_) => {
                                            thread_stats
                                                .relay_write_failures
                                                .fetch_add(1, Ordering::Relaxed);
                                        }
                                    }
                                }
                            }
                        } else {
                            let destination = ipv4_destination(&tun_buffer[..packet_len]);
                            let relay_targets = relay_peers
                                .iter()
                                .map(|peer| {
                                    format!(
                                        "{}:{}",
                                        peer.peer_node_id,
                                        peer.peer_virtual_ips.join("|")
                                    )
                                })
                                .collect::<Vec<_>>()
                                .join(",");
                            let derp_targets = derp_peers
                                .iter()
                                .map(|peer| {
                                    format!(
                                        "{}:{}",
                                        peer.peer_node_id,
                                        peer.peer_virtual_ips.join("|")
                                    )
                                })
                                .collect::<Vec<_>>()
                                .join(",");
                            android_log_error(&format!(
                                "SLAN_ANDROID_FFI_NO_PEER destination={:?} relayTargets=[{}] derpTargets=[{}] directReady={} directAttached={} packet={}",
                                destination,
                                relay_targets,
                                derp_targets,
                                direct_udp
                                    .as_ref()
                                    .map(|transport| transport.ready_peer_count())
                                    .unwrap_or(0),
                                direct_udp
                                    .as_ref()
                                    .map(|transport| transport.peers.len())
                                    .unwrap_or(0),
                                packet_summary(&tun_buffer[..packet_len]),
                            ));
                            if let Ok(mut value) = thread_stats.last_no_peer_packet.lock() {
                                *value = Some(packet_summary(&tun_buffer[..packet_len]));
                            }
                            if let Some(destination) = ipv4_destination(&tun_buffer[..packet_len]) {
                                if should_ignore_unroutable_destination(&destination) {
                                    continue;
                                }
                                if let Ok(mut value) = thread_stats.last_no_peer_destination.lock()
                                {
                                    *value = Some(destination);
                                }
                            }
                            thread_stats
                                .relay_no_peer_packets
                                .fetch_add(1, Ordering::Relaxed);
                        }
                    }
                    Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == ErrorKind::Interrupted => {}
                    Err(_) => break,
                }
                for peer in &relay_peers {
                    match peer.socket.recv(&mut relay_buffer) {
                        Ok(frame_len) => {
                            did_work = true;
                            android_log_info(&format!(
                                "SLAN_ANDROID_FFI_RELAY_RECV_RAW sessionId={} peerNodeId={} len={} prefix={}",
                                peer.session_id,
                                peer.peer_node_id,
                                frame_len,
                                String::from_utf8_lossy(
                                    &relay_buffer[..frame_len.min(200)]
                                ),
                            ));
                            let decoded_payload = relay_packet_payload(&relay_buffer[..frame_len]);
                            if decoded_payload.is_none() {
                                if let Some(kind) = relay_control_kind(&relay_buffer[..frame_len]) {
                                    android_log_info(&format!(
                                        "SLAN_ANDROID_FFI_RELAY_FRAME_CONTROL sessionId={} peerNodeId={} len={} kind={} frame={}",
                                        peer.session_id,
                                        peer.peer_node_id,
                                        frame_len,
                                        kind,
                                        String::from_utf8_lossy(&relay_buffer[..frame_len]),
                                    ));
                                    thread_stats
                                        .relay_control_packets_received
                                        .fetch_add(1, Ordering::Relaxed);
                                    if let Ok(mut value) =
                                        thread_stats.last_relay_control_kind.lock()
                                    {
                                        *value = Some(kind);
                                    }
                                    continue;
                                }
                                android_log_error(&format!(
                                    "SLAN_ANDROID_FFI_RELAY_FRAME_DECODE_FAIL sessionId={} peerNodeId={} len={} kind={:?} frame={}",
                                    peer.session_id,
                                    peer.peer_node_id,
                                    frame_len,
                                    relay_frame_kind(&relay_buffer[..frame_len]),
                                    String::from_utf8_lossy(&relay_buffer[..frame_len]),
                                ));
                                thread_stats
                                    .relay_decode_failures
                                    .fetch_add(1, Ordering::Relaxed);
                                if let Ok(mut value) = thread_stats.last_relay_raw_frame.lock() {
                                    *value = Some(
                                        String::from_utf8_lossy(&relay_buffer[..frame_len])
                                            .into_owned(),
                                    );
                                }
                                continue;
                            }
                            let frame = decoded_payload
                                .as_deref()
                                .unwrap_or(&relay_buffer[..frame_len]);
                            android_log_info(&format!(
                                "SLAN_ANDROID_FFI_RELAY_FRAME_PACKET sessionId={} peerNodeId={} outerLen={} innerLen={}",
                                peer.session_id,
                                peer.peer_node_id,
                                frame_len,
                                frame.len(),
                            ));
                            if let Some(packet) = decode_slan_relay_data_frame(frame) {
                                thread_stats
                                    .relay_frames_received
                                    .fetch_add(1, Ordering::Relaxed);
                                android_log_info(&format!(
                                    "SLAN_ANDROID_FFI_RELAY_FRAME_DATA_OK sessionId={} peerNodeId={} packet={}",
                                    peer.session_id,
                                    peer.peer_node_id,
                                    packet_summary(packet),
                                ));
                                record_relay_tcp_packet(&thread_stats, packet);
                                if !acl_allows_ingress_packet(
                                    packet,
                                    &acl_policies,
                                    Some(&acl_peer_for_relay_peer(peer)),
                                ) {
                                    android_log_info(&format!(
                                        "SLAN_ANDROID_FFI_RELAY_FRAME_ACL_DROP sessionId={} peerNodeId={} packet={}",
                                        peer.session_id,
                                        peer.peer_node_id,
                                        packet_summary(packet),
                                    ));
                                    continue;
                                }
                                if let Some(reply) =
                                    icmp_echo_reply_for_request(packet, local_virtual_ip.as_str())
                                {
                                    seq = seq.wrapping_add(1);
                                    if let Some(frame) =
                                        encode_slan_relay_data_frame(seq, config_hash, &reply)
                                    {
                                        if let Some(payload) = encode_relay_forward(peer, &frame) {
                                            match peer.socket.send(&payload) {
                                                Ok(_) => {
                                                    thread_stats
                                                        .relay_frames_sent
                                                        .fetch_add(1, Ordering::Relaxed);
                                                }
                                                Err(_) => {
                                                    thread_stats
                                                        .relay_write_failures
                                                        .fetch_add(1, Ordering::Relaxed);
                                                }
                                            }
                                        }
                                    }
                                    continue;
                                }
                                let packet = normalize_ipv4_transport_checksums(packet);
                                if !acl_allows_ingress_packet(
                                    &packet,
                                    &acl_policies,
                                    Some(&acl_peer_for_relay_peer(peer)),
                                ) {
                                    android_log_info(&format!(
                                        "SLAN_ANDROID_FFI_RELAY_FRAME_ACL_DROP_NORMALIZED sessionId={} peerNodeId={} packet={}",
                                        peer.session_id,
                                        peer.peer_node_id,
                                        packet_summary(&packet),
                                    ));
                                    continue;
                                }
                                android_log_info(&format!(
                                    "SLAN_ANDROID_FFI_RELAY_FRAME_TUN_WRITE sessionId={} peerNodeId={} packet={}",
                                    peer.session_id,
                                    peer.peer_node_id,
                                    packet_summary(&packet),
                                ));
                                write_android_tun_inbound_packet(&mut file, &thread_stats, &packet);
                            } else {
                                android_log_error(&format!(
                                    "SLAN_ANDROID_FFI_RELAY_FRAME_DATA_DECODE_NONE sessionId={} peerNodeId={} frameLen={} framePrefix={}",
                                    peer.session_id,
                                    peer.peer_node_id,
                                    frame.len(),
                                    String::from_utf8_lossy(&frame[..frame.len().min(160)]),
                                ));
                            }
                        }
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
                for peer in &mut derp_peers {
                    match recv_derp_packet(peer) {
                        Ok(Some(frame)) => {
                            did_work = true;
                            if let Some(packet) = decode_slan_relay_data_frame(&frame) {
                                thread_stats
                                    .relay_frames_received
                                    .fetch_add(1, Ordering::Relaxed);
                                record_relay_tcp_packet(&thread_stats, packet);
                                if !acl_allows_ingress_packet(
                                    packet,
                                    &acl_policies,
                                    Some(&acl_peer_for_derp_peer(peer)),
                                ) {
                                    continue;
                                }
                                let packet = normalize_ipv4_transport_checksums(packet);
                                if !acl_allows_ingress_packet(
                                    &packet,
                                    &acl_policies,
                                    Some(&acl_peer_for_derp_peer(peer)),
                                ) {
                                    continue;
                                }
                                write_android_tun_inbound_packet(&mut file, &thread_stats, &packet);
                            }
                        }
                        Ok(None) => {}
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
                if let Some(direct_udp) = direct_udp.as_mut() {
                    match direct_udp.recv_from_peer(&mut relay_buffer) {
                        Ok(Some(received)) => {
                            did_work = true;
                            let frame_len = received.frame_len;
                            let control_packet =
                                direct_udp_control_packet(&relay_buffer[..frame_len]);
                            if control_packet
                                .as_ref()
                                .is_some_and(|packet| packet.kind == DirectUdpControlKind::Probe)
                            {
                                direct_udp
                                    .mark_peer_ready(received.peer_index, received.remote_addr);
                                thread_stats
                                    .direct_udp_ready_peer_count
                                    .store(direct_udp.ready_peer_count() as u64, Ordering::Relaxed);
                                thread_stats
                                    .direct_udp_probes_received
                                    .fetch_add(1, Ordering::Relaxed);
                                if direct_udp.send_pong_to_peer(received.peer_index) {
                                    thread_stats
                                        .direct_udp_pongs_sent
                                        .fetch_add(1, Ordering::Relaxed);
                                }
                                continue;
                            }
                            if control_packet
                                .as_ref()
                                .is_some_and(|packet| packet.kind == DirectUdpControlKind::Pong)
                            {
                                direct_udp
                                    .mark_peer_ready(received.peer_index, received.remote_addr);
                                thread_stats
                                    .direct_udp_ready_peer_count
                                    .store(direct_udp.ready_peer_count() as u64, Ordering::Relaxed);
                                thread_stats
                                    .direct_udp_pongs_received
                                    .fetch_add(1, Ordering::Relaxed);
                                continue;
                            }
                            if let Some(packet) =
                                decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                            {
                                direct_udp
                                    .mark_peer_ready(received.peer_index, received.remote_addr);
                                thread_stats
                                    .direct_udp_ready_peer_count
                                    .store(direct_udp.ready_peer_count() as u64, Ordering::Relaxed);
                                thread_stats
                                    .direct_udp_frames_received
                                    .fetch_add(1, Ordering::Relaxed);
                                record_relay_tcp_packet(&thread_stats, packet);
                                let acl_peer = direct_udp
                                    .peers
                                    .get(received.peer_index)
                                    .map(acl_peer_for_direct_peer);
                                if !acl_allows_ingress_packet(
                                    packet,
                                    &acl_policies,
                                    acl_peer.as_ref(),
                                ) {
                                    continue;
                                }
                                if let Some(reply) =
                                    icmp_echo_reply_for_request(packet, local_virtual_ip.as_str())
                                {
                                    seq = seq.wrapping_add(1);
                                    if let Some(frame) =
                                        encode_slan_relay_data_frame(seq, config_hash, &reply)
                                    {
                                        let _ =
                                            direct_udp.send_to_peer(received.peer_index, &frame);
                                    }
                                    continue;
                                }
                                let packet = normalize_ipv4_transport_checksums(packet);
                                if !acl_allows_ingress_packet(
                                    &packet,
                                    &acl_policies,
                                    acl_peer.as_ref(),
                                ) {
                                    continue;
                                }
                                write_android_tun_inbound_packet(&mut file, &thread_stats, &packet);
                            }
                        }
                        Ok(None) => {}
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
                if !did_work {
                    thread::sleep(DATA_PLANE_IDLE_SLEEP);
                }
            }
            detach_udp_relay_sessions(&relay_peers, parsed_config.as_ref(), &thread_stats);
            detach_derp_relay_sessions(&mut derp_peers);
        });
        let mut guard = runtime()
            .lock()
            .expect("android tun runtime mutex poisoned");
        *guard = Some(TunRuntime {
            stop,
            stats,
            last_attach_error,
            relay_peer_virtual_ips,
            derp_peer_virtual_ips,
            relay_session_ids,
            handle: Some(handle),
            config_json,
        });
        Ok(())
    }

    fn write_tun_packet_with_retry(file: &mut File, packet: &[u8]) -> std::io::Result<()> {
        let deadline = Instant::now() + Duration::from_secs(1);
        loop {
            match file.write(packet) {
                Ok(0) => {
                    if Instant::now() >= deadline {
                        return Err(std::io::Error::new(
                            ErrorKind::WriteZero,
                            "android tun write made no progress",
                        ));
                    }
                    thread::sleep(Duration::from_millis(2));
                }
                Ok(written) if written == packet.len() => return Ok(()),
                Ok(written) => {
                    return Err(std::io::Error::new(
                        ErrorKind::WriteZero,
                        format!(
                            "android tun short write: wrote {written} of {} bytes",
                            packet.len()
                        ),
                    ));
                }
                Err(error) if error.kind() == ErrorKind::WouldBlock => {
                    if Instant::now() >= deadline {
                        return Err(error);
                    }
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => return Err(error),
            }
        }
    }

    fn write_android_tun_inbound_packet(file: &mut File, stats: &TunStats, packet: &[u8]) {
        for packet in android_tun_inbound_packets(packet) {
            let repeat_count = if is_ipv4_udp_packet(&packet) { 3 } else { 1 };
            for attempt in 0..repeat_count {
                if attempt > 0 {
                    thread::sleep(DATA_PLANE_IDLE_SLEEP);
                }
                if let Err(error) = write_tun_packet_with_retry(file, &packet) {
                    stats.tun_write_failures.fetch_add(1, Ordering::Relaxed);
                    record_tun_write_failure(stats, &packet, &error);
                } else {
                    record_tun_write_packet(stats, &packet);
                    stats
                        .bytes_written
                        .fetch_add(packet.len() as u64, Ordering::Relaxed);
                }
            }
        }
    }

    fn android_tun_inbound_packet(packet: &[u8]) -> Vec<u8> {
        let mut packet = normalize_ipv4_transport_checksums(packet);
        clear_ipv4_udp_checksum(&mut packet);
        packet
    }

    fn android_tun_inbound_packets(packet: &[u8]) -> Vec<Vec<u8>> {
        if let Some((payload_packet, fin_packet)) = split_tcp_fin_payload_packet(packet) {
            return vec![
                android_tun_inbound_packet(&payload_packet),
                android_tun_inbound_packet(&fin_packet),
            ];
        }
        vec![android_tun_inbound_packet(packet)]
    }

    fn split_tcp_fin_payload_packet(packet: &[u8]) -> Option<(Vec<u8>, Vec<u8>)> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
            return None;
        }
        let ip_header_len = usize::from(packet[0] & 0x0f) * 4;
        if ip_header_len < 20 || packet.len() < ip_header_len + 20 {
            return None;
        }
        let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
        if total_len > packet.len() || total_len < ip_header_len + 20 {
            return None;
        }
        let tcp_offset = ip_header_len;
        let tcp_header_len = usize::from(packet[tcp_offset + 12] >> 4) * 4;
        let tcp_payload_offset = tcp_offset.checked_add(tcp_header_len)?;
        if tcp_header_len < 20 || tcp_payload_offset > total_len {
            return None;
        }
        let payload_len = total_len.saturating_sub(tcp_payload_offset);
        let flags = packet[tcp_offset + 13];
        if flags & 0x01 == 0 || payload_len == 0 {
            return None;
        }

        let mut payload_packet = packet[..total_len].to_vec();
        payload_packet[tcp_offset + 13] = flags & !0x01;

        let mut fin_packet = packet[..tcp_payload_offset].to_vec();
        let seq = u32::from_be_bytes([
            packet[tcp_offset + 4],
            packet[tcp_offset + 5],
            packet[tcp_offset + 6],
            packet[tcp_offset + 7],
        ]);
        let fin_seq = seq.wrapping_add(payload_len as u32).to_be_bytes();
        fin_packet[tcp_offset + 4..tcp_offset + 8].copy_from_slice(&fin_seq);
        fin_packet[tcp_offset + 13] = (flags & !0x08) | 0x01;
        let fin_total_len = fin_packet.len() as u16;
        fin_packet[2..4].copy_from_slice(&fin_total_len.to_be_bytes());
        Some((payload_packet, fin_packet))
    }

    fn clear_ipv4_udp_checksum(packet: &mut [u8]) {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(17) {
            return;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 8 {
            return;
        }
        packet[ihl + 6] = 0;
        packet[ihl + 7] = 0;
    }

    fn encode_relay_forward(peer: &AndroidRelayPeer, frame: &[u8]) -> Option<Vec<u8>> {
        serde_json::to_vec(&serde_json::json!({
            "kind": "forward",
            "session_id": peer.session_id,
            "participant_id": peer.local_node_id,
            "payload": base64_encode(frame),
        }))
        .ok()
    }

    fn hedge_udp_packet_to_relay(
        relay_peers: &[AndroidRelayPeer],
        derp_peers: &mut [AndroidDerpPeer],
        packet: &[u8],
        frame: &[u8],
        stats: &TunStats,
    ) -> bool {
        let mut sent = false;
        if let Some(peer) = relay_peer_for_packet(relay_peers, packet) {
            if let Some(payload) = encode_relay_forward(peer, frame) {
                android_log_info(&format!(
                    "SLAN_ANDROID_FFI_RELAY_HEDGE_SEND sessionId={} peerNodeId={} payloadBytes={} frameBytes={} packet={}",
                    peer.session_id,
                    peer.peer_node_id,
                    payload.len(),
                    frame.len(),
                    packet_summary(packet),
                ));
                match peer.socket.send(&payload) {
                    Ok(_) => {
                        sent = true;
                        android_log_info(&format!(
                            "SLAN_ANDROID_FFI_RELAY_HEDGE_SEND_OK sessionId={} peerNodeId={} payloadBytes={}",
                            peer.session_id,
                            peer.peer_node_id,
                            payload.len(),
                        ));
                        stats.relay_frames_sent.fetch_add(1, Ordering::Relaxed);
                    }
                    Err(error) => {
                        android_log_error(&format!(
                            "SLAN_ANDROID_FFI_RELAY_HEDGE_SEND_ERROR sessionId={} peerNodeId={} payloadBytes={} error={}",
                            peer.session_id,
                            peer.peer_node_id,
                            payload.len(),
                            error,
                        ));
                        stats.relay_write_failures.fetch_add(1, Ordering::Relaxed);
                    }
                }
            }
        }
        if let Some(peer) = derp_peer_for_packet_mut(derp_peers, packet) {
            match send_derp_forward(peer, frame) {
                Ok(_) => {
                    sent = true;
                    stats.relay_frames_sent.fetch_add(1, Ordering::Relaxed);
                }
                Err(_) => {
                    stats.relay_write_failures.fetch_add(1, Ordering::Relaxed);
                }
            }
        }
        sent
    }

    fn send_relay_keepalives(peers: &[AndroidRelayPeer]) {
        for peer in peers {
            let Ok(payload) = serde_json::to_vec(&serde_json::json!({
                "kind": "ping",
                "session_id": peer.session_id,
                "participant_id": peer.local_node_id,
            })) else {
                continue;
            };
            let _ = peer.socket.send(&payload);
        }
    }

    fn relay_packet_payload(frame: &[u8]) -> Option<Vec<u8>> {
        let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
        if value.get("kind").and_then(serde_json::Value::as_str) != Some("packet") {
            return None;
        }
        base64_decode(value.get("payload")?.as_str()?)
    }

    fn relay_frame_kind(frame: &[u8]) -> Option<String> {
        let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
        value
            .get("kind")
            .and_then(serde_json::Value::as_str)
            .map(str::to_string)
    }

    fn relay_control_kind(frame: &[u8]) -> Option<String> {
        let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
        match value.get("kind").and_then(serde_json::Value::as_str)? {
            "pong" | "attached" | "forwarded" | "detached" | "error" => value
                .get("kind")
                .and_then(serde_json::Value::as_str)
                .map(str::to_string),
            _ => None,
        }
    }

    fn tun_stats_json(runtime: &TunRuntime) -> String {
        serde_json::json!({
            "running": true,
            "requestedRelaySessionCount": runtime.stats.requested_relay_session_count.load(Ordering::Relaxed),
            "attachedRelaySessionCount": runtime.stats.attached_relay_session_count.load(Ordering::Relaxed),
            "relayAttachFailures": runtime.stats.relay_attach_failures.load(Ordering::Relaxed),
            "lastRelayAttachError": runtime
                .last_attach_error
                .lock()
                .ok()
                .and_then(|value| value.clone()),
            "packetsRead": runtime.stats.packets_read.load(Ordering::Relaxed),
            "bytesRead": runtime.stats.bytes_read.load(Ordering::Relaxed),
            "bytesWritten": runtime.stats.bytes_written.load(Ordering::Relaxed),
            "packetsTooLarge": runtime.stats.packets_too_large.load(Ordering::Relaxed),
            "relayFramesSent": runtime.stats.relay_frames_sent.load(Ordering::Relaxed),
            "relayFramesReceived": runtime.stats.relay_frames_received.load(Ordering::Relaxed),
            "relayControlPacketsReceived": runtime.stats.relay_control_packets_received.load(Ordering::Relaxed),
            "relayDecodeFailures": runtime.stats.relay_decode_failures.load(Ordering::Relaxed),
            "relayTcpFramesReceived": runtime.stats.relay_tcp_frames_received.load(Ordering::Relaxed),
            "relayTcpSynAckReceived": runtime.stats.relay_tcp_syn_ack_received.load(Ordering::Relaxed),
            "relayTcpPshReceived": runtime.stats.relay_tcp_psh_received.load(Ordering::Relaxed),
            "relayTcpRstReceived": runtime.stats.relay_tcp_rst_received.load(Ordering::Relaxed),
            "relayTcpChecksumInvalid": runtime.stats.relay_tcp_checksum_invalid.load(Ordering::Relaxed),
            "relayDetachSent": runtime.stats.relay_detach_sent.load(Ordering::Relaxed),
            "relayNoPeerPackets": runtime.stats.relay_no_peer_packets.load(Ordering::Relaxed),
            "directUdpAttachedPeerCount": runtime.stats.direct_udp_attached_peer_count.load(Ordering::Relaxed),
            "directUdpReadyPeerCount": runtime.stats.direct_udp_ready_peer_count.load(Ordering::Relaxed),
            "directUdpProbesSent": runtime.stats.direct_udp_probes_sent.load(Ordering::Relaxed),
            "directUdpProbesReceived": runtime.stats.direct_udp_probes_received.load(Ordering::Relaxed),
            "directUdpPongsSent": runtime.stats.direct_udp_pongs_sent.load(Ordering::Relaxed),
            "directUdpPongsReceived": runtime.stats.direct_udp_pongs_received.load(Ordering::Relaxed),
            "directUdpFramesSent": runtime.stats.direct_udp_frames_sent.load(Ordering::Relaxed),
            "directUdpFramesReceived": runtime.stats.direct_udp_frames_received.load(Ordering::Relaxed),
            "relayPeerVirtualIps": runtime
                .relay_peer_virtual_ips
                .lock()
                .ok()
                .map(|value| value.clone())
                .unwrap_or_default(),
            "derpPeerVirtualIps": runtime
                .derp_peer_virtual_ips
                .lock()
                .ok()
                .map(|value| value.clone())
                .unwrap_or_default(),
            "relaySessionIds": runtime
                .relay_session_ids
                .lock()
                .ok()
                .map(|value| value.clone())
                .unwrap_or_default(),
            "lastNoPeerDestination": runtime.stats.last_no_peer_destination.lock().ok().and_then(|value| value.clone()),
            "lastNoPeerPacket": runtime.stats.last_no_peer_packet.lock().ok().and_then(|value| value.clone()),
            "relayWriteFailures": runtime.stats.relay_write_failures.load(Ordering::Relaxed),
            "tunWriteFailures": runtime.stats.tun_write_failures.load(Ordering::Relaxed),
            "lastRelayControlKind": runtime.stats.last_relay_control_kind.lock().ok().and_then(|value| value.clone()),
            "lastRelayRawFrame": runtime.stats.last_relay_raw_frame.lock().ok().and_then(|value| value.clone()),
            "lastTunWriteError": runtime.stats.last_tun_write_error.lock().ok().and_then(|value| value.clone()),
            "lastTunWritePacket": runtime.stats.last_tun_write_packet.lock().ok().and_then(|value| value.clone()),
            "configBytes": runtime.config_json.len(),
        })
        .to_string()
    }

    fn requested_relay_session_count(config: Option<&AndroidVpnSessionConfig>) -> u64 {
        config
            .and_then(|value| value.relay_data_plane.as_ref())
            .filter(|relay_config| relay_config.enabled)
            .map(|relay_config| relay_config.sessions.len() as u64)
            .unwrap_or(0)
    }

    fn record_relay_tcp_packet(stats: &TunStats, packet: &[u8]) {
        let Some(flags) = ipv4_tcp_flags(packet) else {
            return;
        };
        stats
            .relay_tcp_frames_received
            .fetch_add(1, Ordering::Relaxed);
        if flags & 0x12 == 0x12 {
            stats
                .relay_tcp_syn_ack_received
                .fetch_add(1, Ordering::Relaxed);
        }
        if flags & 0x08 != 0 {
            stats.relay_tcp_psh_received.fetch_add(1, Ordering::Relaxed);
        }
        if flags & 0x04 != 0 {
            stats.relay_tcp_rst_received.fetch_add(1, Ordering::Relaxed);
        }
        if ipv4_transport_checksum_valid(packet) == Some(false) {
            stats
                .relay_tcp_checksum_invalid
                .fetch_add(1, Ordering::Relaxed);
        }
    }

    fn relay_send_attempt_count(packet: &[u8]) -> usize {
        if let Some(flags) = ipv4_tcp_flags(packet) {
            if flags & 0x12 == 0x02 {
                4
            } else if flags & 0x0b != 0 || ipv4_tcp_payload_len(packet).unwrap_or(0) > 0 {
                2
            } else {
                1
            }
        } else if is_ipv4_udp_packet(packet) {
            3
        } else {
            1
        }
    }

    fn should_hedge_direct_packet_to_relay(packet: &[u8]) -> bool {
        if is_ipv4_udp_packet(packet) {
            return true;
        }
        let Some(flags) = ipv4_tcp_flags(packet) else {
            return false;
        };
        flags & 0x12 == 0x02 || flags & 0x0b != 0 || ipv4_tcp_payload_len(packet).unwrap_or(0) > 0
    }

    fn relay_send_attempt_delay(packet: &[u8]) -> Duration {
        if is_ipv4_tcp_initial_syn(packet) {
            Duration::from_millis(30)
        } else {
            DATA_PLANE_IDLE_SLEEP
        }
    }

    fn is_ipv4_tcp_initial_syn(packet: &[u8]) -> bool {
        ipv4_tcp_flags(packet)
            .map(|flags| flags & 0x12 == 0x02)
            .unwrap_or(false)
    }

    fn record_tun_write_failure(stats: &TunStats, packet: &[u8], error: &std::io::Error) {
        if let Ok(mut value) = stats.last_tun_write_error.lock() {
            *value = Some(format!("kind={:?}; error={}", error.kind(), error));
        }
        record_tun_write_packet(stats, packet);
    }

    fn record_tun_write_packet(stats: &TunStats, packet: &[u8]) {
        if let Ok(mut value) = stats.last_tun_write_packet.lock() {
            *value = Some(packet_summary(packet));
        }
    }

    fn packet_summary(packet: &[u8]) -> String {
        let protocol = packet.get(9).copied().unwrap_or_default();
        let flags = ipv4_tcp_flags(packet)
            .map(|value| format!("0x{value:02x}"))
            .unwrap_or_else(|| "none".to_string());
        let (source_port, destination_port) = ipv4_transport_ports(packet)
            .map(|(source, destination)| (source.to_string(), destination.to_string()))
            .unwrap_or_else(|| ("unknown".to_string(), "unknown".to_string()));
        let checksum_valid = ipv4_transport_checksum_valid(packet)
            .map(|value| value.to_string())
            .unwrap_or_else(|| "unknown".to_string());
        let first_byte = packet
            .first()
            .map(|value| format!("0x{value:02x}"))
            .unwrap_or_else(|| "none".to_string());
        let tcp = tcp_summary(packet).unwrap_or_else(|| "tcp=none".to_string());
        format!(
            "len={}; firstByte={}; protocol={}; src={}; dst={}; srcPort={}; dstPort={}; tcpFlags={}; checksumValid={}; {}",
            packet.len(),
            first_byte,
            protocol,
            ipv4_source(packet).unwrap_or_else(|| "unknown".to_string()),
            ipv4_destination(packet).unwrap_or_else(|| "unknown".to_string()),
            source_port,
            destination_port,
            flags,
            checksum_valid,
            tcp
        )
    }

    fn tcp_summary(packet: &[u8]) -> Option<String> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 20 {
            return None;
        }
        let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
        if total_len < ihl + 20 || total_len > packet.len() {
            return None;
        }
        let data_offset = usize::from(packet[ihl + 12] >> 4) * 4;
        if data_offset < 20 || total_len < ihl + data_offset {
            return None;
        }
        let seq = u32::from_be_bytes([
            packet[ihl + 4],
            packet[ihl + 5],
            packet[ihl + 6],
            packet[ihl + 7],
        ]);
        let ack = u32::from_be_bytes([
            packet[ihl + 8],
            packet[ihl + 9],
            packet[ihl + 10],
            packet[ihl + 11],
        ]);
        let payload_len = total_len.saturating_sub(ihl + data_offset);
        Some(format!(
            "tcpSeq={seq}; tcpAck={ack}; tcpDataOffset={data_offset}; tcpPayloadLen={payload_len}"
        ))
    }

    fn ipv4_tcp_flags(packet: &[u8]) -> Option<u8> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 14 {
            return None;
        }
        Some(packet[ihl + 13])
    }

    fn ipv4_tcp_payload_len(packet: &[u8]) -> Option<usize> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 20 {
            return None;
        }
        let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
        if total_len < ihl + 20 || total_len > packet.len() {
            return None;
        }
        let data_offset = usize::from(packet[ihl + 12] >> 4) * 4;
        if data_offset < 20 || total_len < ihl + data_offset {
            return None;
        }
        Some(total_len - ihl - data_offset)
    }

    fn ipv4_tcp_ports(packet: &[u8]) -> Option<(u16, u16)> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 4 {
            return None;
        }
        Some((
            u16::from_be_bytes([packet[ihl], packet[ihl + 1]]),
            u16::from_be_bytes([packet[ihl + 2], packet[ihl + 3]]),
        ))
    }

    fn ipv4_transport_ports(packet: &[u8]) -> Option<(u16, u16)> {
        if packet.len() < 20 || packet[0] >> 4 != 4 {
            return None;
        }
        match packet.get(9).copied() {
            Some(6) => ipv4_tcp_ports(packet),
            Some(17) => ipv4_udp_ports(packet),
            _ => None,
        }
    }

    fn ipv4_udp_ports(packet: &[u8]) -> Option<(u16, u16)> {
        if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(17) {
            return None;
        }
        let ihl = usize::from(packet[0] & 0x0f) * 4;
        if ihl < 20 || packet.len() < ihl + 8 {
            return None;
        }
        Some((
            u16::from_be_bytes([packet[ihl], packet[ihl + 1]]),
            u16::from_be_bytes([packet[ihl + 2], packet[ihl + 3]]),
        ))
    }

    fn is_ipv4_udp_packet(packet: &[u8]) -> bool {
        packet.len() >= 20 && packet[0] >> 4 == 4 && packet.get(9).copied() == Some(17)
    }

    fn read_relay_fds(env: &mut JNIEnv, relay_fds: JIntArray) -> jni::errors::Result<Vec<c_int>> {
        let len = env.get_array_length(&relay_fds)?;
        if len <= 0 {
            return Ok(Vec::new());
        }
        let mut values = vec![0_i32; len as usize];
        env.get_int_array_region(&relay_fds, 0, &mut values)?;
        Ok(values.into_iter().filter(|fd| *fd >= 0).collect())
    }

    fn prepare_relay_sockets(
        relay_fds: Vec<c_int>,
        config: Option<&AndroidVpnSessionConfig>,
        last_attach_error: Arc<Mutex<Option<String>>>,
    ) -> std::io::Result<(
        Vec<AndroidRelayPeer>,
        Vec<AndroidDerpPeer>,
        Option<DirectUdpTransport>,
    )> {
        if relay_fds.is_empty() {
            return Ok((Vec::new(), Vec::new(), None));
        }
        let Some(relay_config) = config.and_then(|value| value.relay_data_plane.as_ref()) else {
            close_relay_fds(relay_fds);
            return Ok((Vec::new(), Vec::new(), None));
        };
        if !relay_config.enabled {
            close_relay_fds(relay_fds);
            return Ok((Vec::new(), Vec::new(), None));
        };
        let mut peers = Vec::new();
        let mut derp_peers = Vec::new();
        let mut relay_fds = relay_fds.into_iter();
        for index in 0..relay_config.sessions.len() {
            let Some(fd) = relay_fds.next() else {
                break;
            };
            let Some(session) = relay_config.sessions.get(index) else {
                close_relay_fds(vec![fd]);
                continue;
            };
            android_log_info(&format!(
                "SLAN_ANDROID_FFI_PREPARE_RELAY_FD index={} fd={} sessionId={} peerNodeId={} relayUrl={} localNodeId={} peerVirtualIps={:?}",
                index,
                fd,
                session.session_id,
                session.peer_node_id,
                session.ticket.relay_url,
                relay_config.local_node_id,
                session.peer_virtual_ips,
            ));
            if is_derp_session(session) {
                let stream = unsafe { TcpStream::from_raw_fd(fd) };
                match attach_derp_relay_session(
                    stream,
                    relay_config.local_node_id.as_str(),
                    session,
                ) {
                    Ok(peer) => {
                        android_log_info(&format!(
                            "SLAN_ANDROID_FFI_ATTACHED_DERP index={} fd={} sessionId={} peerNodeId={} serverSessionId={}",
                            index,
                            fd,
                            session.session_id,
                            session.peer_node_id,
                            peer.server_session_id,
                        ));
                        derp_peers.push(peer)
                    }
                    Err(error) => {
                        android_log_error(&format!(
                            "SLAN_ANDROID_FFI_ATTACH_DERP_ERROR index={} fd={} sessionId={} peerNodeId={} error={}",
                            index,
                            fd,
                            session.session_id,
                            session.peer_node_id,
                            error,
                        ));
                        if let Ok(mut guard) = last_attach_error.lock() {
                            *guard = Some(error.to_string());
                        }
                    }
                }
                continue;
            }
            let socket = unsafe { UdpSocket::from_raw_fd(fd) };
            if let Err(error) =
                attach_udp_relay_session(&socket, relay_config.local_node_id.as_str(), session)
            {
                android_log_error(&format!(
                    "SLAN_ANDROID_FFI_ATTACH_UDP_ERROR index={} fd={} sessionId={} peerNodeId={} relayUrl={} error={}",
                    index,
                    fd,
                    session.session_id,
                    session.peer_node_id,
                    session.ticket.relay_url,
                    error,
                ));
                if let Ok(mut guard) = last_attach_error.lock() {
                    *guard = Some(error.to_string());
                }
                continue;
            }
            socket.set_nonblocking(true)?;
            let local_addr = socket.local_addr().ok();
            let peer_addr = socket.peer_addr().ok();
            android_log_info(&format!(
                "SLAN_ANDROID_FFI_ATTACHED_UDP index={} fd={} sessionId={} peerNodeId={} relayUrl={} localAddr={:?} peerAddr={:?} peerVirtualIps={:?}",
                index,
                fd,
                session.session_id,
                session.peer_node_id,
                session.ticket.relay_url,
                local_addr,
                peer_addr,
                session.peer_virtual_ips,
            ));
            peers.push(AndroidRelayPeer {
                session_id: session.session_id.clone(),
                peer_node_id: session.peer_node_id.clone(),
                local_node_id: relay_config.local_node_id.clone(),
                peer_virtual_ips: session.peer_virtual_ips.clone(),
                socket,
            });
        }
        let direct_udp = relay_fds.next().and_then(|fd| {
            let socket = unsafe { UdpSocket::from_raw_fd(fd) };
            let local_addr = socket.local_addr().ok();
            android_log_info(&format!(
                "SLAN_ANDROID_FFI_PREPARE_DIRECT_UDP fd={} localAddr={:?} localNodeId={} peerPathCount={} nodeConfigCount={}",
                fd,
                local_addr,
                relay_config.local_node_id,
                relay_config.peer_paths.len(),
                relay_config.node_configs.len(),
            ));
            DirectUdpTransport::attach_with_socket(
                relay_config.local_node_id.as_str(),
                &relay_config.peer_paths,
                || Ok(socket),
            )
        });
        if direct_udp.is_some() {
            android_log_info("SLAN_ANDROID_FFI_ATTACHED_DIRECT_UDP ready");
        } else {
            android_log_error("SLAN_ANDROID_FFI_ATTACHED_DIRECT_UDP missing");
        }
        close_relay_fds(relay_fds.collect());
        Ok((peers, derp_peers, direct_udp))
    }

    fn close_relay_fds(relay_fds: Vec<c_int>) {
        for fd in relay_fds {
            let _ = unsafe { UdpSocket::from_raw_fd(fd) };
        }
    }

    fn detach_udp_relay_sessions(
        peers: &[AndroidRelayPeer],
        config: Option<&AndroidVpnSessionConfig>,
        stats: &TunStats,
    ) {
        let local_node_id = config
            .and_then(|value| value.relay_data_plane.as_ref())
            .map(|value| value.local_node_id.as_str())
            .unwrap_or_default();
        if local_node_id.is_empty() {
            return;
        }
        for peer in peers {
            if peer.session_id.is_empty() {
                continue;
            }
            let detach = serde_json::json!({
                "kind": "detach",
                "session_id": &peer.session_id,
                "participant_id": local_node_id,
            });
            if let Ok(payload) = serde_json::to_vec(&detach) {
                if peer.socket.send(&payload).is_ok() {
                    stats.relay_detach_sent.fetch_add(1, Ordering::Relaxed);
                }
            }
        }
    }

    fn attach_udp_relay_session(
        socket: &UdpSocket,
        local_node_id: &str,
        session: &RelayPeerSession,
    ) -> std::io::Result<()> {
        socket.set_nonblocking(false)?;
        socket.set_read_timeout(Some(Duration::from_secs(2)))?;
        socket.set_write_timeout(Some(Duration::from_secs(2)))?;
        let attach = serde_json::json!({
            "kind": "attach",
            "participant_id": local_node_id,
            "ticket": relay_ticket_wire(session),
        });
        let payload = serde_json::to_vec(&attach).map_err(json_error)?;
        let mut response = vec![0_u8; 4096];
        let mut last_error = None;
        for _ in 0..3 {
            if let Err(error) = socket.send(&payload) {
                last_error = Some(error);
                thread::sleep(Duration::from_millis(250));
                continue;
            }
            match socket.recv(&mut response) {
                Ok(len) => {
                    verify_relay_attach_ack(&response[..len], &session.session_id).map_err(|error| {
                        std::io::Error::new(
                            error.kind(),
                            format!(
                                "relay attach ack rejected sessionId={} relayUrl={} localNodeId={} peerNodeId={} response={}",
                                session.session_id,
                                session.ticket.relay_url,
                                local_node_id,
                                session.peer_node_id,
                                String::from_utf8_lossy(&response[..len])
                            ),
                        )
                    })?;
                    socket.set_read_timeout(None)?;
                    socket.set_write_timeout(None)?;
                    return Ok(());
                }
                Err(error)
                    if error.kind() == ErrorKind::WouldBlock
                        || error.kind() == ErrorKind::TimedOut =>
                {
                    last_error = Some(error);
                    thread::sleep(Duration::from_millis(250));
                }
                Err(error) => {
                    last_error = Some(error);
                    thread::sleep(Duration::from_millis(250));
                }
            }
        }
        Err(last_error
            .unwrap_or_else(|| std::io::Error::new(ErrorKind::TimedOut, "relay attach timed out")))
    }

    fn attach_derp_relay_session(
        stream: TcpStream,
        local_node_id: &str,
        session: &RelayPeerSession,
    ) -> std::io::Result<AndroidDerpPeer> {
        stream.set_nodelay(true)?;
        stream.set_nonblocking(false)?;
        stream.set_read_timeout(Some(Duration::from_secs(3)))?;
        stream.set_write_timeout(Some(Duration::from_secs(3)))?;
        let mut writer = stream.try_clone()?;
        let mut reader = BufReader::new(stream.try_clone()?);
        let connect = serde_json::json!({
            "kind": "connect",
            "peerId": local_node_id,
            "nodeId": derp_ticket_node_id(session),
            "regionId": derp_ticket_region_id(session),
            "ticket": derp_ticket_wire(local_node_id, session),
        });
        write_json_line(&mut writer, &connect)?;
        let line = read_derp_connect_line_with_retry(&mut reader, Duration::from_secs(3))?;
        let value: serde_json::Value = serde_json::from_str(line.trim()).map_err(json_error)?;
        if value.get("kind").and_then(serde_json::Value::as_str) != Some("connected") {
            return Err(std::io::Error::new(
                ErrorKind::Other,
                value
                    .get("error")
                    .and_then(|error| error.get("message"))
                    .and_then(serde_json::Value::as_str)
                    .unwrap_or("DERP connect failed")
                    .to_string(),
            ));
        }
        let server_session_id = value
            .get("sessionId")
            .and_then(serde_json::Value::as_str)
            .unwrap_or(session.session_id.as_str())
            .to_string();
        writer.set_nonblocking(true)?;
        let reader = reader.into_inner();
        reader.set_nonblocking(true)?;
        Ok(AndroidDerpPeer {
            peer_node_id: session.peer_node_id.clone(),
            local_node_id: local_node_id.to_string(),
            server_session_id,
            peer_virtual_ips: session.peer_virtual_ips.clone(),
            stream: writer,
            reader,
            read_buffer: Vec::new(),
        })
    }

    fn is_derp_session(session: &RelayPeerSession) -> bool {
        let relay_url = session.ticket.relay_url.trim().to_ascii_lowercase();
        relay_url.starts_with("derp://")
            || relay_url.starts_with("derp+tcp+tls://")
            || relay_url.starts_with("derp_tcp_tls_443://")
    }

    fn send_derp_forward(peer: &mut AndroidDerpPeer, frame: &[u8]) -> std::io::Result<()> {
        let payload = serde_json::json!({
            "kind": "send",
            "sessionId": peer.server_session_id,
            "targetPeerId": peer.peer_node_id,
            "payload": base64_encode(frame),
        });
        write_json_line(&mut peer.stream, &payload)
    }

    fn recv_derp_packet(peer: &mut AndroidDerpPeer) -> std::io::Result<Option<Vec<u8>>> {
        if let Some(line) = take_derp_line(&mut peer.read_buffer) {
            return parse_derp_packet_line(&line);
        }
        let mut chunk = [0_u8; 4096];
        loop {
            match peer.reader.read(&mut chunk) {
                Ok(0) => {
                    return Err(std::io::Error::new(
                        ErrorKind::ConnectionReset,
                        "DERP TCP connection closed",
                    ));
                }
                Ok(len) => {
                    peer.read_buffer.extend_from_slice(&chunk[..len]);
                    if let Some(line) = take_derp_line(&mut peer.read_buffer) {
                        return parse_derp_packet_line(&line);
                    }
                }
                Err(error) if error.kind() == ErrorKind::WouldBlock => return Ok(None),
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => return Err(error),
            }
        }
    }

    fn take_derp_line(buffer: &mut Vec<u8>) -> Option<Vec<u8>> {
        let newline = buffer.iter().position(|byte| *byte == b'\n')?;
        let mut line = buffer.drain(..=newline).collect::<Vec<_>>();
        while matches!(line.last(), Some(b'\n' | b'\r')) {
            line.pop();
        }
        Some(line)
    }

    fn parse_derp_packet_line(line: &[u8]) -> std::io::Result<Option<Vec<u8>>> {
        let value: serde_json::Value = serde_json::from_slice(line).map_err(json_error)?;
        match value.get("kind").and_then(serde_json::Value::as_str) {
            Some("recv") => Ok(value
                .get("payload")
                .and_then(serde_json::Value::as_str)
                .and_then(base64_decode)),
            Some("sent") | Some("connected") => Ok(None),
            Some("error") => Err(std::io::Error::new(
                ErrorKind::Other,
                value
                    .get("error")
                    .and_then(|error| error.get("message"))
                    .and_then(serde_json::Value::as_str)
                    .unwrap_or("DERP error")
                    .to_string(),
            )),
            _ => Ok(None),
        }
    }

    fn detach_derp_relay_sessions(peers: &mut [AndroidDerpPeer]) {
        for peer in peers {
            let payload = serde_json::json!({
                "kind": "disconnect",
                "sessionId": peer.server_session_id,
                "peerId": peer.local_node_id,
            });
            let _ = write_json_line(&mut peer.stream, &payload);
        }
    }

    fn write_json_line(stream: &mut TcpStream, value: &serde_json::Value) -> std::io::Result<()> {
        let mut payload = serde_json::to_vec(value).map_err(json_error)?;
        payload.push(b'\n');
        write_all_with_would_block_retry(stream, &payload, DERP_WRITE_RETRY_TIMEOUT)
    }

    fn read_derp_connect_line_with_retry<R: BufRead>(
        reader: &mut R,
        timeout: Duration,
    ) -> std::io::Result<String> {
        let started = Instant::now();
        loop {
            let mut line = String::new();
            match reader.read_line(&mut line) {
                Ok(0) => {
                    return Err(std::io::Error::new(
                        ErrorKind::ConnectionReset,
                        "DERP TCP connection closed before connect ack",
                    ));
                }
                Ok(_) => return Ok(line),
                Err(error)
                    if error.kind() == ErrorKind::WouldBlock
                        || error.kind() == ErrorKind::TimedOut =>
                {
                    if started.elapsed() >= timeout {
                        return Err(error);
                    }
                    thread::sleep(Duration::from_millis(10));
                }
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => return Err(error),
            }
        }
    }

    fn write_all_with_would_block_retry(
        stream: &mut TcpStream,
        payload: &[u8],
        timeout: Duration,
    ) -> std::io::Result<()> {
        let started = Instant::now();
        let mut offset = 0;
        while offset < payload.len() {
            match stream.write(&payload[offset..]) {
                Ok(0) => {
                    return Err(std::io::Error::new(
                        ErrorKind::WriteZero,
                        "DERP TCP write returned zero bytes",
                    ));
                }
                Ok(written) => offset += written,
                Err(error) if error.kind() == ErrorKind::WouldBlock => {
                    if started.elapsed() >= timeout {
                        return Err(error);
                    }
                    thread::sleep(Duration::from_millis(2));
                }
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => return Err(error),
            }
        }
        Ok(())
    }

    fn derp_ticket_node_id(session: &RelayPeerSession) -> String {
        session
            .ticket
            .allowed_derp_node_ids
            .iter()
            .map(|value| value.trim())
            .find(|value| !value.is_empty())
            .unwrap_or("derp")
            .to_string()
    }

    fn derp_ticket_region_id(session: &RelayPeerSession) -> String {
        session
            .ticket
            .derp_cluster_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .unwrap_or("default")
            .to_string()
    }

    fn derp_ticket_wire(local_node_id: &str, session: &RelayPeerSession) -> serde_json::Value {
        let ticket = &session.ticket;
        serde_json::json!({
            "ticketId": &ticket.ticket_id,
            "peerId": local_node_id,
            "networkId": &ticket.network_id,
            "path": "derp_tcp_tls_443",
            "regionId": derp_ticket_region_id(session),
            "nodeId": derp_ticket_node_id(session),
            "sessionId": &ticket.session_id,
            "srcNodeId": &ticket.src_node_id,
            "dstNodeId": &ticket.dst_node_id,
            "relayUrl": &ticket.relay_url,
            "sessionKey": &ticket.session_key,
            "allowedDerpNodeIds": &ticket.allowed_derp_node_ids,
            "expiresAt": &ticket.expires_at,
            "signature": &ticket.signature,
        })
    }

    fn relay_ticket_wire(session: &RelayPeerSession) -> serde_json::Value {
        let ticket = &session.ticket;
        serde_json::json!({
            "ticketId": &ticket.ticket_id,
            "networkId": &ticket.network_id,
            "sessionId": &ticket.session_id,
            "srcNodeId": &ticket.src_node_id,
            "dstNodeId": &ticket.dst_node_id,
            "derpClusterId": &ticket.derp_cluster_id,
            "countryCode": &ticket.country_code,
            "cityCode": &ticket.city_code,
            "allowedDerpNodeIds": &ticket.allowed_derp_node_ids,
            "relayUrl": &ticket.relay_url,
            "expiresAt": &ticket.expires_at,
            "sessionKey": &ticket.session_key,
            "signature": &ticket.signature,
        })
    }

    fn verify_relay_attach_ack(response: &[u8], session_id: &str) -> std::io::Result<()> {
        let value: serde_json::Value = serde_json::from_slice(response).map_err(json_error)?;
        match value.get("kind").and_then(serde_json::Value::as_str) {
            Some("attached") => {
                let ack_session = value
                    .get("session_id")
                    .or_else(|| value.get("sessionId"))
                    .and_then(serde_json::Value::as_str)
                    .unwrap_or_default();
                if ack_session == session_id {
                    Ok(())
                } else {
                    Err(std::io::Error::new(
                        ErrorKind::InvalidData,
                        "relay attach session mismatch",
                    ))
                }
            }
            Some("error") => Err(std::io::Error::new(
                ErrorKind::PermissionDenied,
                value
                    .get("message")
                    .and_then(serde_json::Value::as_str)
                    .unwrap_or("relay attach failed")
                    .to_string(),
            )),
            _ => Err(std::io::Error::new(
                ErrorKind::InvalidData,
                "unexpected relay attach response",
            )),
        }
    }

    fn relay_peer_for_packet<'a>(
        peers: &'a [AndroidRelayPeer],
        packet: &[u8],
    ) -> Option<&'a AndroidRelayPeer> {
        let Some(destination) = ipv4_destination(packet) else {
            return None;
        };
        peers.iter().find(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == destination)
        })
    }

    fn acl_peer_for_relay_peer(peer: &AndroidRelayPeer) -> PlatformAclPeer {
        PlatformAclPeer {
            peer_node_id: Some(peer.peer_node_id.clone()),
            peer_virtual_ips: peer.peer_virtual_ips.clone(),
        }
    }

    fn acl_peer_for_derp_peer(peer: &AndroidDerpPeer) -> PlatformAclPeer {
        PlatformAclPeer {
            peer_node_id: Some(peer.peer_node_id.clone()),
            peer_virtual_ips: peer.peer_virtual_ips.clone(),
        }
    }

    fn acl_peer_for_direct_peer(
        peer: &client_core_platform::direct_udp::DirectUdpPeer,
    ) -> PlatformAclPeer {
        PlatformAclPeer {
            peer_node_id: Some(peer.peer_node_id.clone()),
            peer_virtual_ips: peer.peer_virtual_ips.clone(),
        }
    }

    fn derp_peer_for_packet_mut<'a>(
        peers: &'a mut [AndroidDerpPeer],
        packet: &[u8],
    ) -> Option<&'a mut AndroidDerpPeer> {
        let destination = ipv4_destination(packet)?;
        peers.iter_mut().find(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == destination)
        })
    }

    fn packet_targets_local_virtual_ip(packet: &[u8], local_virtual_ip: &str) -> bool {
        ipv4_destination(packet)
            .map(|destination| destination == normalize_virtual_ip(local_virtual_ip))
            .unwrap_or(false)
    }

    fn local_dns_reply(
        packet: &[u8],
        dns_servers: &[String],
        dns_records: &[PlatformResolverRecord],
    ) -> Option<Vec<u8>> {
        for dns_server in dns_servers {
            if let Some(reply) = resolver_response_for_query(packet, dns_server, dns_records) {
                return Some(reply);
            }
        }
        None
    }

    fn local_virtual_ip_reply(packet: &[u8], local_virtual_ip: &str) -> Option<Vec<u8>> {
        icmp_echo_reply_for_request(packet, local_virtual_ip)
    }

    fn should_ignore_unroutable_destination(destination: &str) -> bool {
        let mut parts = destination
            .split('.')
            .filter_map(|part| part.parse::<u8>().ok());
        let Some(first) = parts.next() else {
            return false;
        };
        first >= 224 || destination == "255.255.255.255"
    }

    fn ipv4_destination(packet: &[u8]) -> Option<String> {
        if packet.len() < 20 || packet[0] >> 4 != 4 {
            return None;
        }
        Some(format!(
            "{}.{}.{}.{}",
            packet[16], packet[17], packet[18], packet[19]
        ))
    }

    fn ipv4_source(packet: &[u8]) -> Option<String> {
        if packet.len() < 20 || packet[0] >> 4 != 4 {
            return None;
        }
        Some(format!(
            "{}.{}.{}.{}",
            packet[12], packet[13], packet[14], packet[15]
        ))
    }

    fn normalize_virtual_ip(value: &str) -> String {
        value
            .trim()
            .split_once('/')
            .map(|(ip, _)| ip)
            .unwrap_or_else(|| value.trim())
            .to_string()
    }

    fn json_error(error: serde_json::Error) -> std::io::Error {
        std::io::Error::new(ErrorKind::InvalidData, error)
    }

    fn stop_tun() {
        let mut guard = runtime()
            .lock()
            .expect("android tun runtime mutex poisoned");
        let _ = guard.take();
    }

    fn set_nonblocking(fd: c_int) {
        unsafe {
            let flags = libc::fcntl(fd, libc::F_GETFL, 0);
            if flags >= 0 {
                let _ = libc::fcntl(fd, libc::F_SETFL, flags | libc::O_NONBLOCK);
            }
        }
    }
}

#[no_mangle]
pub extern "C" fn client_core_v2_version() -> *mut c_char {
    string_to_ptr(env!("CARGO_PKG_VERSION").to_string())
}

#[no_mangle]
pub extern "C" fn client_core_v2_default_state_json() -> *mut c_char {
    match serde_json::to_string(&ClientViewState::default()) {
        Ok(value) => string_to_ptr(value),
        Err(_) => ptr::null_mut(),
    }
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn client_core_v2_initialize(state_dir: *const c_char) -> *mut c_char {
    let state_dir = if state_dir.is_null() {
        ""
    } else {
        unsafe { CStr::from_ptr(state_dir) }
            .to_str()
            .unwrap_or_default()
    };
    string_to_ptr(embedded_initialize_json(state_dir))
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn client_core_v2_service_request_json(request_json: *const c_char) -> *mut c_char {
    if request_json.is_null() {
        return string_to_ptr(
            serde_json::json!({
                "error": "request_json is null",
            })
            .to_string(),
        );
    }
    let request = unsafe { CStr::from_ptr(request_json) }
        .to_string_lossy()
        .into_owned();
    string_to_ptr(client_core_service::embedded::embedded_handle_request_json(
        &request,
    ))
}

#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn client_core_v2_free_string(value: *mut c_char) {
    if value.is_null() {
        return;
    }
    unsafe {
        drop(CString::from_raw(value));
    }
}

fn string_to_ptr(value: String) -> *mut c_char {
    match CString::new(value) {
        Ok(value) => value.into_raw(),
        Err(_) => ptr::null_mut(),
    }
}

fn embedded_initialize_json(state_dir: &str) -> String {
    match client_core_service::embedded::embedded_initialize(Some(state_dir)) {
        Ok(value) => value.to_string(),
        Err(error) => serde_json::json!({ "error": format!("{error:#}") }).to_string(),
    }
}
