use std::{
    ffi::{c_char, CString},
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
            atomic::{AtomicBool, Ordering},
            Arc, Mutex, OnceLock,
        },
        thread::{self, JoinHandle},
        time::Duration,
    };

    use client_core::relay_frame::{
        decode_slan_relay_data_frame, encode_slan_relay_data_frame, stable_hash64,
    };
    use client_core::{AndroidVpnSessionConfig, RelayPeerSession};
    use jni::{
        objects::{JClass, JIntArray, JString},
        JNIEnv,
    };

    struct TunRuntime {
        stop: Arc<AtomicBool>,
        handle: Option<JoinHandle<()>>,
        config_json: String,
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

    struct AndroidRelayPeer {
        session_id: String,
        peer_virtual_ips: Vec<String>,
        socket: UdpSocket,
    }

    fn start_tun(tun_fd: c_int, relay_fds: Vec<c_int>, config_json: String) -> std::io::Result<()> {
        stop_tun();
        set_nonblocking(tun_fd);
        let parsed_config = serde_json::from_str::<AndroidVpnSessionConfig>(&config_json).ok();
        let relay_peers = prepare_relay_sockets(relay_fds, parsed_config.as_ref())?;
        let max_frame_payload = parsed_config
            .as_ref()
            .and_then(|config| config.relay_data_plane.as_ref())
            .and_then(|config| config.max_frame_payload)
            .unwrap_or(1200)
            .clamp(512, 1400) as usize;
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            let mut file = unsafe { File::from_raw_fd(tun_fd) };
            let relay_peers = relay_peers;
            let config_hash = stable_hash64(&config_json);
            let mut seq = 0_u64;
            let mut tun_buffer = vec![0_u8; 2048];
            let mut relay_buffer = vec![0_u8; 4096];
            while !thread_stop.load(Ordering::SeqCst) {
                match file.read(&mut tun_buffer) {
                    Ok(0) => thread::sleep(Duration::from_millis(20)),
                    Ok(packet_len) => {
                        if packet_len > max_frame_payload {
                            continue;
                        }
                        if let Some(peer) =
                            relay_peer_for_packet(&relay_peers, &tun_buffer[..packet_len])
                        {
                            seq = seq.wrapping_add(1);
                            if let Some(frame) = encode_slan_relay_data_frame(
                                seq,
                                config_hash,
                                &tun_buffer[..packet_len],
                            ) {
                                let _ = peer.socket.send(&frame);
                            }
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
                            if let Some(packet) =
                                decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                            {
                                let _ = file.write_all(packet);
                            }
                        }
                        Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                        Err(error) if error.kind() == ErrorKind::Interrupted => {}
                        Err(_) => {}
                    }
                }
            }
            detach_udp_relay_sessions(&relay_peers, parsed_config.as_ref());
        });
        let mut guard = runtime()
            .lock()
            .expect("android tun runtime mutex poisoned");
        *guard = Some(TunRuntime {
            stop,
            handle: Some(handle),
            config_json,
        });
        Ok(())
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
    ) -> std::io::Result<Vec<AndroidRelayPeer>> {
        if relay_fds.is_empty() {
            return Ok(Vec::new());
        }
        let Some(relay_config) = config.and_then(|value| value.relay_data_plane.as_ref()) else {
            return relay_fds
                .into_iter()
                .map(|fd| {
                    let socket = unsafe { UdpSocket::from_raw_fd(fd) };
                    socket.set_nonblocking(true)?;
                    Ok(AndroidRelayPeer {
                        session_id: String::new(),
                        peer_virtual_ips: Vec::new(),
                        socket,
                    })
                })
                .collect();
        };
        if !relay_config.enabled
            || !relay_config.transport.eq_ignore_ascii_case("udp")
            || relay_config.sessions.is_empty()
        {
            close_relay_fds(relay_fds);
            return Ok(Vec::new());
        };
        let mut peers = Vec::new();
        for (index, fd) in relay_fds.into_iter().enumerate() {
            let Some(session) = relay_config.sessions.get(index) else {
                close_relay_fds(vec![fd]);
                continue;
            };
            let socket = unsafe { UdpSocket::from_raw_fd(fd) };
            attach_udp_relay_session(&socket, relay_config.local_node_id.as_str(), session)?;
            socket.set_nonblocking(true)?;
            peers.push(AndroidRelayPeer {
                session_id: session.session_id.clone(),
                peer_virtual_ips: session.peer_virtual_ips.clone(),
                socket,
            });
        }
        Ok(peers)
    }

    fn close_relay_fds(relay_fds: Vec<c_int>) {
        for fd in relay_fds {
            let _ = unsafe { UdpSocket::from_raw_fd(fd) };
        }
    }

    fn detach_udp_relay_sessions(
        peers: &[AndroidRelayPeer],
        config: Option<&AndroidVpnSessionConfig>,
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
                let _ = peer.socket.send(&payload);
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
        if peers.is_empty() {
            return None;
        }
        if peers.len() == 1 {
            return Some(&peers[0]);
        }
        let Some(destination) = ipv4_destination(packet) else {
            return None;
        };
        peers.iter().find(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == destination)
        })
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
