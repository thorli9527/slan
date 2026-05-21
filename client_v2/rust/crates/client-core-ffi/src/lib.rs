use std::{
    ffi::{c_char, CStr, CString},
    ptr,
};

use client_core::ClientViewState;

#[cfg(target_os = "android")]
mod android_tun {
    use std::{
        fs::File,
        io::{ErrorKind, Read, Write},
        net::UdpSocket,
        os::fd::FromRawFd,
        os::raw::c_int,
        sync::{
            atomic::{AtomicBool, AtomicU64, Ordering},
            Arc, Mutex, OnceLock,
        },
        thread::{self, JoinHandle},
        time::{Duration, Instant},
    };

    use client_core::relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    };
    use client_core::{
        icmp_echo_reply_for_request, ipv4_transport_checksum_valid,
        normalize_ipv4_transport_checksums, AndroidVpnSessionConfig, RelayPeerSession,
    };
    use client_core_platform::direct_udp::{
        direct_udp_control_packet, direct_udp_probe_interval_from_ms, DirectUdpControlKind,
        DirectUdpTransport,
    };
    use jni::{
        objects::{JClass, JIntArray, JString},
        JNIEnv,
    };

    const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(30);

    /// TunRuntime 持有 Android TUN fd 数据面线程、运行统计和原始配置。
    struct TunRuntime {
        stop: Arc<AtomicBool>,
        stats: Arc<TunStats>,
        last_attach_error: Arc<Mutex<Option<String>>>,
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

    /// AndroidRelayPeer 是 Rust 数据面中一个 relay peer 会话及其受保护 socket。
    struct AndroidRelayPeer {
        session_id: String,
        local_node_id: String,
        peer_virtual_ips: Vec<String>,
        socket: UdpSocket,
    }

    /// 启动 Android TUN 数据面，接管 Java 层 detach 出来的 TUN fd 和 UDP socket fd。
    fn start_tun(tun_fd: c_int, relay_fds: Vec<c_int>, config_json: String) -> std::io::Result<()> {
        stop_tun();
        set_nonblocking(tun_fd);
        let parsed_config = serde_json::from_str::<AndroidVpnSessionConfig>(&config_json).ok();
        let stats = Arc::new(TunStats::default());
        let last_attach_error = Arc::new(Mutex::new(None));
        stats.requested_relay_session_count.store(
            requested_relay_session_count(parsed_config.as_ref()),
            Ordering::Relaxed,
        );
        let (relay_peers, direct_udp) = prepare_udp_sockets(
            relay_fds,
            parsed_config.as_ref(),
            Arc::clone(&last_attach_error),
        )?;
        stats.direct_udp_attached_peer_count.store(
            direct_udp
                .as_ref()
                .map(|transport| transport.peers.len() as u64)
                .unwrap_or(0),
            Ordering::Relaxed,
        );
        stats
            .attached_relay_session_count
            .store(relay_peers.len() as u64, Ordering::Relaxed);
        stats.relay_attach_failures.store(
            stats
                .requested_relay_session_count
                .load(Ordering::Relaxed)
                .saturating_sub(relay_peers.len() as u64),
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
            let config_hash = stable_hash64(&thread_config_json);
            let mut seq = 0_u64;
            let mut tun_buffer = vec![0_u8; 2048];
            let mut relay_buffer = vec![0_u8; 4096];
            let mut direct_udp = direct_udp;
            let direct_udp_probe_interval = parsed_config
                .as_ref()
                .and_then(|config| config.relay_data_plane.as_ref())
                .map(|config| {
                    direct_udp_probe_interval_from_ms(config.path_policy.probe_interval_ms)
                })
                .unwrap_or_else(|| Duration::from_secs(15));
            let mut last_direct_udp_probe = Instant::now()
                .checked_sub(direct_udp_probe_interval)
                .unwrap_or_else(Instant::now);
            let mut last_keepalive = Instant::now()
                .checked_sub(RELAY_KEEPALIVE_INTERVAL)
                .unwrap_or_else(Instant::now);
            while !thread_stop.load(Ordering::SeqCst) {
                if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
                    if let Some(direct_udp) = direct_udp.as_ref() {
                        thread_stats
                            .direct_udp_probes_sent
                            .fetch_add(direct_udp.send_probe_packets() as u64, Ordering::Relaxed);
                    }
                    last_direct_udp_probe = Instant::now();
                }
                if last_keepalive.elapsed() >= RELAY_KEEPALIVE_INTERVAL {
                    send_relay_keepalives(&relay_peers);
                    last_keepalive = Instant::now();
                }
                match file.read(&mut tun_buffer) {
                    Ok(0) => thread::sleep(Duration::from_millis(20)),
                    Ok(packet_len) => {
                        thread_stats.packets_read.fetch_add(1, Ordering::Relaxed);
                        thread_stats
                            .bytes_read
                            .fetch_add(packet_len as u64, Ordering::Relaxed);
                        if packet_targets_local_virtual_ip(
                            &tun_buffer[..packet_len],
                            local_virtual_ip.as_str(),
                        ) {
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
                                seq = seq.wrapping_add(1);
                                let packet =
                                    normalize_ipv4_transport_checksums(&tun_buffer[..packet_len]);
                                if let Some(frame) =
                                    encode_slan_relay_data_frame(seq, config_hash, &packet)
                                {
                                    if direct_udp.send_to_peer(peer_index, &frame).is_ok() {
                                        thread_stats
                                            .direct_udp_frames_sent
                                            .fetch_add(1, Ordering::Relaxed);
                                        continue;
                                    }
                                }
                            }
                        }
                        if let Some(peer) =
                            relay_peer_for_packet(&relay_peers, &tun_buffer[..packet_len])
                        {
                            seq = seq.wrapping_add(1);
                            let packet =
                                normalize_ipv4_transport_checksums(&tun_buffer[..packet_len]);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
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
                                } else {
                                    thread_stats
                                        .relay_write_failures
                                        .fetch_add(1, Ordering::Relaxed);
                                }
                            }
                        } else {
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
                    Err(error) if error.kind() == ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(20));
                    }
                    Err(error) if error.kind() == ErrorKind::Interrupted => {}
                    Err(_) => break,
                }
                for peer in &relay_peers {
                    match peer.socket.recv(&mut relay_buffer) {
                        Ok(frame_len) => {
                            let decoded_payload = relay_packet_payload(&relay_buffer[..frame_len]);
                            if decoded_payload.is_none()
                                && relay_control_kind(&relay_buffer[..frame_len]).is_some()
                            {
                                continue;
                            }
                            let frame = decoded_payload
                                .as_deref()
                                .unwrap_or(&relay_buffer[..frame_len]);
                            if let Some(packet) = decode_slan_relay_data_frame(frame) {
                                thread_stats
                                    .relay_frames_received
                                    .fetch_add(1, Ordering::Relaxed);
                                record_relay_tcp_packet(&thread_stats, packet);
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
                                if let Err(error) = write_tun_packet_with_retry(&mut file, &packet)
                                {
                                    thread_stats
                                        .tun_write_failures
                                        .fetch_add(1, Ordering::Relaxed);
                                    record_tun_write_failure(&thread_stats, &packet, &error);
                                } else {
                                    record_tun_write_packet(&thread_stats, &packet);
                                    thread_stats
                                        .bytes_written
                                        .fetch_add(packet.len() as u64, Ordering::Relaxed);
                                }
                            }
                        }
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
                if let Some(direct_udp) = direct_udp.as_mut() {
                    match direct_udp.recv_from_peer(&mut relay_buffer) {
                        Ok(Some(received)) => {
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
                                if let Err(error) = write_tun_packet_with_retry(&mut file, &packet)
                                {
                                    thread_stats
                                        .tun_write_failures
                                        .fetch_add(1, Ordering::Relaxed);
                                    record_tun_write_failure(&thread_stats, &packet, &error);
                                } else {
                                    record_tun_write_packet(&thread_stats, &packet);
                                    thread_stats
                                        .bytes_written
                                        .fetch_add(packet.len() as u64, Ordering::Relaxed);
                                }
                            }
                        }
                        Ok(None) => {}
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
            }
            detach_udp_relay_sessions(&relay_peers, parsed_config.as_ref(), &thread_stats);
        });
        let mut guard = runtime()
            .lock()
            .expect("android tun runtime mutex poisoned");
        *guard = Some(TunRuntime {
            stop,
            stats,
            last_attach_error,
            handle: Some(handle),
            config_json,
        });
        Ok(())
    }

    fn write_tun_packet_with_retry(file: &mut File, packet: &[u8]) -> std::io::Result<()> {
        let deadline = Instant::now() + Duration::from_secs(1);
        let mut offset = 0_usize;
        while offset < packet.len() {
            match file.write(&packet[offset..]) {
                Ok(0) => {
                    if Instant::now() >= deadline {
                        return Err(std::io::Error::new(
                            ErrorKind::WriteZero,
                            "android tun write made no progress",
                        ));
                    }
                    thread::sleep(Duration::from_millis(2));
                }
                Ok(written) => offset += written,
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
        Ok(())
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
            "lastNoPeerDestination": runtime.stats.last_no_peer_destination.lock().ok().and_then(|value| value.clone()),
            "lastNoPeerPacket": runtime.stats.last_no_peer_packet.lock().ok().and_then(|value| value.clone()),
            "relayWriteFailures": runtime.stats.relay_write_failures.load(Ordering::Relaxed),
            "tunWriteFailures": runtime.stats.tun_write_failures.load(Ordering::Relaxed),
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
        let (source_port, destination_port) = ipv4_tcp_ports(packet)
            .map(|(source, destination)| (source.to_string(), destination.to_string()))
            .unwrap_or_else(|| ("unknown".to_string(), "unknown".to_string()));
        let checksum_valid = ipv4_transport_checksum_valid(packet)
            .map(|value| value.to_string())
            .unwrap_or_else(|| "unknown".to_string());
        let first_byte = packet
            .first()
            .map(|value| format!("0x{value:02x}"))
            .unwrap_or_else(|| "none".to_string());
        format!(
            "len={}; firstByte={}; protocol={}; src={}; dst={}; srcPort={}; dstPort={}; tcpFlags={}; checksumValid={}",
            packet.len(),
            first_byte,
            protocol,
            ipv4_source(packet).unwrap_or_else(|| "unknown".to_string()),
            ipv4_destination(packet).unwrap_or_else(|| "unknown".to_string()),
            source_port,
            destination_port,
            flags,
            checksum_valid
        )
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

    fn read_relay_fds(env: &mut JNIEnv, relay_fds: JIntArray) -> jni::errors::Result<Vec<c_int>> {
        let len = env.get_array_length(&relay_fds)?;
        if len <= 0 {
            return Ok(Vec::new());
        }
        let mut values = vec![0_i32; len as usize];
        env.get_int_array_region(&relay_fds, 0, &mut values)?;
        Ok(values.into_iter().filter(|fd| *fd >= 0).collect())
    }

    fn prepare_udp_sockets(
        relay_fds: Vec<c_int>,
        config: Option<&AndroidVpnSessionConfig>,
        last_attach_error: Arc<Mutex<Option<String>>>,
    ) -> std::io::Result<(Vec<AndroidRelayPeer>, Option<DirectUdpTransport>)> {
        if relay_fds.is_empty() {
            return Ok((Vec::new(), None));
        }
        let Some(relay_config) = config.and_then(|value| value.relay_data_plane.as_ref()) else {
            close_relay_fds(relay_fds);
            return Ok((Vec::new(), None));
        };
        if !relay_config.enabled || !relay_config.transport.eq_ignore_ascii_case("udp") {
            close_relay_fds(relay_fds);
            return Ok((Vec::new(), None));
        };
        let mut peers = Vec::new();
        let mut relay_fds = relay_fds.into_iter();
        for index in 0..relay_config.sessions.len() {
            let Some(fd) = relay_fds.next() else {
                break;
            };
            let Some(session) = relay_config.sessions.get(index) else {
                close_relay_fds(vec![fd]);
                continue;
            };
            let socket = unsafe { UdpSocket::from_raw_fd(fd) };
            if let Err(error) =
                attach_udp_relay_session(&socket, relay_config.local_node_id.as_str(), session)
            {
                if let Ok(mut guard) = last_attach_error.lock() {
                    *guard = Some(error.to_string());
                }
                continue;
            }
            socket.set_nonblocking(true)?;
            peers.push(AndroidRelayPeer {
                session_id: session.session_id.clone(),
                local_node_id: relay_config.local_node_id.clone(),
                peer_virtual_ips: session.peer_virtual_ips.clone(),
                socket,
            });
        }
        let direct_udp = relay_fds.next().and_then(|fd| {
            let socket = unsafe { UdpSocket::from_raw_fd(fd) };
            DirectUdpTransport::attach_with_socket(
                relay_config.local_node_id.as_str(),
                &relay_config.peer_paths,
                || Ok(socket),
            )
        });
        close_relay_fds(relay_fds.collect());
        Ok((peers, direct_udp))
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
                    verify_relay_attach_ack(&response[..len], &session.session_id)?;
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

    fn relay_ticket_wire(session: &RelayPeerSession) -> serde_json::Value {
        let ticket = &session.ticket;
        serde_json::json!({
            "ticket_id": &ticket.ticket_id,
            "network_id": &ticket.network_id,
            "session_id": &ticket.session_id,
            "src_node_id": &ticket.src_node_id,
            "dst_node_id": &ticket.dst_node_id,
            "derp_cluster_id": &ticket.derp_cluster_id,
            "country_code": &ticket.country_code,
            "city_code": &ticket.city_code,
            "allowed_derp_node_ids": &ticket.allowed_derp_node_ids,
            "relay_url": &ticket.relay_url,
            "expires_at": &ticket.expires_at,
            "session_key": &ticket.session_key,
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

    fn packet_targets_local_virtual_ip(packet: &[u8], local_virtual_ip: &str) -> bool {
        ipv4_destination(packet)
            .map(|destination| destination == normalize_virtual_ip(local_virtual_ip))
            .unwrap_or(false)
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
