package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	frameHeaderLen = 32
	frameVersion   = byte(1)
	frameTypeData  = byte(1)
)

var frameMagic = []byte("SLAN")

type iosClient struct {
	name string
	id   string
	ip   string
	dns  string

	udp4     *net.UDPConn
	udp6     *net.UDPConn
	udp4Addr *net.UDPAddr
	udp6Addr *net.UDPAddr

	inbox chan receivedPacket
}

type receivedPacket struct {
	protocol string
	fromID   string
	srcIP    string
	dstIP    string
	body     string
}

type packetEnvelope struct {
	protocol string
	fromID   string
	srcIP    string
	dstIP    string
	body     string
}

type routeTarget struct {
	client *iosClient
	addr   *net.UDPAddr
}

type udpRelay struct {
	conn    *net.UDPConn
	targets map[string]routeTarget
}

type tcpRelay struct {
	listener net.Listener
	targets  map[string]*iosClient
}

func main() {
	var timeout time.Duration
	var platform string
	flag.DurationVar(&timeout, "timeout", 20*time.Second, "test timeout")
	flag.StringVar(&platform, "platform", "ios", "logical client platform label: ios or android")
	flag.Parse()
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "ios" && platform != "android" {
		fail("unsupported platform %q", platform)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clients := newPlatformClients(platform)
	defer func() {
		for _, client := range clients {
			client.close()
		}
	}()
	for _, client := range clients {
		go client.readUDP(client.udp4)
		go client.readUDP(client.udp6)
	}

	relay := newUDPRelay(clients)
	defer relay.close()
	go relay.serve(ctx)

	derp := newTCPRelay(clients)
	defer derp.close()
	go derp.serve(ctx)

	dnsIndex := map[string]*iosClient{}
	for _, client := range clients {
		dnsIndex[client.dns] = client
	}

	protocols := []string{"lan_udp", "ipv6_udp", "direct_udp", "relay_udp", "derp_tcp_tls_443"}
	for _, protocol := range protocols {
		for _, from := range clients {
			for _, to := range clients {
				if from == to {
					continue
				}
				ipBody := fmt.Sprintf("ip_ping:%s:%s->%s:%s:%d", protocol, from.ip, to.ip, from.id, time.Now().UnixNano())
				if err := sendByProtocol(protocol, from, to, relay, derp, ipBody); err != nil {
					fail("send ip ping %s %s -> %s: %v", protocol, from.id, to.id, err)
				}
				if err := waitPacket(ctx, to, protocol, from.id, from.ip, to.ip, ipBody); err != nil {
					fail("receive ip ping %s %s -> %s: %v", protocol, from.id, to.id, err)
				}
				fmt.Printf("%sTriClientPacketMatrix: ping=ip delivered protocol=%s from=%s srcIP=%s target=%s dstIP=%s body=%s\n", platform, protocol, from.id, from.ip, to.id, to.ip, ipBody)

				resolved := dnsIndex[to.dns]
				if resolved == nil || resolved.ip != to.ip {
					fail("resolve dns %s: got=%v expected=%s", to.dns, resolved, to.ip)
				}
				dnsBody := fmt.Sprintf("dns_ping:%s:%s=>%s:%s->%s:%d", protocol, to.dns, to.ip, from.id, to.id, time.Now().UnixNano())
				if err := sendByProtocol(protocol, from, resolved, relay, derp, dnsBody); err != nil {
					fail("send dns ping %s %s -> %s: %v", protocol, from.id, to.dns, err)
				}
				if err := waitPacket(ctx, to, protocol, from.id, from.ip, to.ip, dnsBody); err != nil {
					fail("receive dns ping %s %s -> %s: %v", protocol, from.id, to.dns, err)
				}
				fmt.Printf("%sTriClientPacketMatrix: ping=dns delivered protocol=%s from=%s srcIP=%s targetName=%s resolvedIP=%s target=%s body=%s\n", platform, protocol, from.id, from.ip, to.dns, to.ip, to.id, dnsBody)
			}
		}
	}

	fmt.Printf("%sTriClientPacketMatrix: ok clients=%s,%s,%s protocols=%s ping=ip,dns\n",
		platform,
		clients[0].id,
		clients[1].id,
		clients[2].id,
		strings.Join(protocols, ","),
	)
}

func newPlatformClients(platform string) []*iosClient {
	switch platform {
	case "ios":
		return []*iosClient{
			newIOSClient("ios-1", "ios-sim-001", "10.0.10.1", "ios-1.mobile.test"),
			newIOSClient("ios-2", "ios-sim-002", "10.0.10.2", "ios-2.mobile.test"),
			newIOSClient("ios-3", "ios-sim-003", "10.0.10.3", "ios-3.mobile.test"),
		}
	case "android":
		return []*iosClient{
			newIOSClient("android-1", "android-sim-001", "10.0.20.1", "android-1.mobile.test"),
			newIOSClient("android-2", "android-sim-002", "10.0.20.2", "android-2.mobile.test"),
			newIOSClient("android-3", "android-sim-003", "10.0.20.3", "android-3.mobile.test"),
		}
	default:
		fail("unsupported platform %q", platform)
		return nil
	}
}

func newIOSClient(name, id, ip, dns string) *iosClient {
	udp4, err := net.ListenUDP("udp4", mustUDPAddr("127.0.0.1:0"))
	if err != nil {
		fail("listen udp4 %s: %v", id, err)
	}
	udp6, err := net.ListenUDP("udp6", mustUDPAddr("[::1]:0"))
	if err != nil {
		_ = udp4.Close()
		fail("listen udp6 %s: %v", id, err)
	}
	return &iosClient{
		name:     name,
		id:       id,
		ip:       ip,
		dns:      dns,
		udp4:     udp4,
		udp6:     udp6,
		udp4Addr: udp4.LocalAddr().(*net.UDPAddr),
		udp6Addr: udp6.LocalAddr().(*net.UDPAddr),
		inbox:    make(chan receivedPacket, 64),
	}
}

func (client *iosClient) close() {
	_ = client.udp4.Close()
	_ = client.udp6.Close()
}

func (client *iosClient) readUDP(conn *net.UDPConn) {
	buffer := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		envelope, err := decodeFrame(buffer[:n])
		if err != nil {
			continue
		}
		client.inbox <- receivedPacket{
			protocol: envelope.protocol,
			fromID:   envelope.fromID,
			srcIP:    envelope.srcIP,
			dstIP:    envelope.dstIP,
			body:     envelope.body,
		}
	}
}

func sendByProtocol(protocol string, from, to *iosClient, relay *udpRelay, derp *tcpRelay, body string) error {
	frame := encodeFrame(packetEnvelope{protocol: protocol, fromID: from.id, srcIP: from.ip, dstIP: to.ip, body: body})
	switch protocol {
	case "lan_udp", "direct_udp":
		_, err := from.udp4.WriteToUDP(frame, to.udp4Addr)
		return err
	case "ipv6_udp":
		_, err := from.udp6.WriteToUDP(frame, to.udp6Addr)
		return err
	case "relay_udp":
		_, err := from.udp4.WriteToUDP(frame, relay.conn.LocalAddr().(*net.UDPAddr))
		return err
	case "derp_tcp_tls_443":
		return derp.send(frame)
	default:
		return fmt.Errorf("unsupported protocol %s", protocol)
	}
}

func waitPacket(ctx context.Context, target *iosClient, protocol, fromID, srcIP, dstIP, body string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case packet := <-target.inbox:
			if packet.protocol == protocol &&
				packet.fromID == fromID &&
				packet.srcIP == srcIP &&
				packet.dstIP == dstIP &&
				packet.body == body {
				return nil
			}
		}
	}
}

func newUDPRelay(clients []*iosClient) *udpRelay {
	conn, err := net.ListenUDP("udp4", mustUDPAddr("127.0.0.1:0"))
	if err != nil {
		fail("listen relay udp: %v", err)
	}
	targets := make(map[string]routeTarget, len(clients))
	for _, client := range clients {
		targets[client.ip] = routeTarget{client: client, addr: client.udp4Addr}
	}
	return &udpRelay{conn: conn, targets: targets}
}

func (relay *udpRelay) close() {
	_ = relay.conn.Close()
}

func (relay *udpRelay) serve(ctx context.Context) {
	buffer := make([]byte, 2048)
	for {
		_ = relay.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _, err := relay.conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
		envelope, err := decodeFrame(buffer[:n])
		if err != nil {
			continue
		}
		target, ok := relay.targets[envelope.dstIP]
		if !ok {
			continue
		}
		_, _ = relay.conn.WriteToUDP(buffer[:n], target.addr)
	}
}

func newTCPRelay(clients []*iosClient) *tcpRelay {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		fail("listen derp tcp: %v", err)
	}
	targets := make(map[string]*iosClient, len(clients))
	for _, client := range clients {
		targets[client.ip] = client
	}
	return &tcpRelay{listener: listener, targets: targets}
}

func (relay *tcpRelay) close() {
	_ = relay.listener.Close()
}

func (relay *tcpRelay) serve(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		if tcpListener, ok := relay.listener.(*net.TCPListener); ok {
			_ = tcpListener.SetDeadline(time.Now().Add(200 * time.Millisecond))
		}
		conn, err := relay.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			frame, err := readLengthPrefixedFrame(conn)
			if err != nil {
				return
			}
			envelope, err := decodeFrame(frame)
			if err != nil {
				return
			}
			target := relay.targets[envelope.dstIP]
			if target == nil {
				return
			}
			target.inbox <- receivedPacket{
				protocol: envelope.protocol,
				fromID:   envelope.fromID,
				srcIP:    envelope.srcIP,
				dstIP:    envelope.dstIP,
				body:     envelope.body,
			}
		}()
	}
}

func (relay *tcpRelay) send(frame []byte) error {
	conn, err := net.DialTimeout("tcp4", relay.listener.Addr().String(), 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(frame)))
	if _, err := conn.Write(length[:]); err != nil {
		return err
	}
	_, err = conn.Write(frame)
	return err
}

func readLengthPrefixedFrame(reader io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(reader, length[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(length[:])
	if size == 0 || size > 4096 {
		return nil, fmt.Errorf("invalid derp frame size %d", size)
	}
	frame := make([]byte, size)
	_, err := io.ReadFull(reader, frame)
	return frame, err
}

func encodeFrame(envelope packetEnvelope) []byte {
	payload := encodeIPv4Payload(envelope)
	frame := make([]byte, 0, frameHeaderLen+len(payload))
	frame = append(frame, frameMagic...)
	frame = append(frame, frameVersion, frameTypeData)
	frame = binary.BigEndian.AppendUint16(frame, frameHeaderLen)
	frame = binary.BigEndian.AppendUint64(frame, uint64(time.Now().UnixNano()))
	frame = binary.BigEndian.AppendUint64(frame, stableHash64(envelope.fromID+"|"+envelope.dstIP))
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(payload)))
	frame = binary.BigEndian.AppendUint32(frame, 0)
	frame = append(frame, payload...)
	return frame
}

func decodeFrame(frame []byte) (packetEnvelope, error) {
	if len(frame) < frameHeaderLen {
		return packetEnvelope{}, fmt.Errorf("frame too short")
	}
	if !bytes.Equal(frame[:4], frameMagic) || frame[4] != frameVersion || frame[5] != frameTypeData {
		return packetEnvelope{}, fmt.Errorf("invalid frame header")
	}
	headerLen := int(binary.BigEndian.Uint16(frame[6:8]))
	payloadLen := int(binary.BigEndian.Uint32(frame[24:28]))
	end := headerLen + payloadLen
	if headerLen < frameHeaderLen || end > len(frame) {
		return packetEnvelope{}, fmt.Errorf("invalid frame length")
	}
	return decodeIPv4Payload(frame[headerLen:end])
}

func encodeIPv4Payload(envelope packetEnvelope) []byte {
	bodyBytes := []byte(envelope.fromID + "\n" + envelope.protocol + "\n" + envelope.body)
	packet := make([]byte, 20+len(bodyBytes))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	copyIPv4(packet[12:16], envelope.srcIP)
	copyIPv4(packet[16:20], envelope.dstIP)
	copy(packet[20:], bodyBytes)
	return packet
}

func decodeIPv4Payload(packet []byte) (packetEnvelope, error) {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		return packetEnvelope{}, fmt.Errorf("not ipv4")
	}
	body := string(packet[20:])
	fromID, rest, ok := strings.Cut(body, "\n")
	if !ok {
		return packetEnvelope{}, fmt.Errorf("missing from id")
	}
	protocol, message, ok := strings.Cut(rest, "\n")
	if !ok {
		return packetEnvelope{}, fmt.Errorf("missing protocol")
	}
	return packetEnvelope{
		protocol: protocol,
		fromID:   fromID,
		srcIP:    fmt.Sprintf("%d.%d.%d.%d", packet[12], packet[13], packet[14], packet[15]),
		dstIP:    fmt.Sprintf("%d.%d.%d.%d", packet[16], packet[17], packet[18], packet[19]),
		body:     message,
	}, nil
}

func copyIPv4(dst []byte, ip string) {
	parsed := net.ParseIP(ip).To4()
	if parsed == nil {
		fail("invalid virtual ip %s", ip)
	}
	copy(dst, parsed)
}

func stableHash64(value string) uint64 {
	var hash uint64 = 0xcbf29ce484222325
	for _, b := range []byte(value) {
		hash ^= uint64(b)
		hash *= 0x100000001b3
	}
	return hash
}

func mustUDPAddr(value string) *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp", value)
	if err != nil {
		fail("resolve udp addr %s: %v", value, err)
	}
	return addr
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
