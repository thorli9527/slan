package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	var udpPort int
	var tcpPort int
	flag.IntVar(&udpPort, "udp-port", 19090, "UDP echo listen port")
	flag.IntVar(&tcpPort, "tcp-port", 19091, "TCP echo listen port")
	flag.Parse()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	if udpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runUDP(udpPort, stop)
		}()
	}
	if tcpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTCP(tcpPort, stop)
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stop)
	wg.Wait()
}

func runUDP(port int, stop <-chan struct{}) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		fmt.Fprintf(os.Stderr, "udp listen failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("SOCKET_ECHO_UDP_READY=%d\n", port)
	buf := make([]byte, 2048)
	for {
		select {
		case <-stop:
			return
		default:
		}
		_ = conn.SetReadDeadline(deadline())
		n, addr, err := conn.ReadFromUDP(buf)
		if timeout(err) {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "udp read failed: %v\n", err)
			continue
		}
		body := string(buf[:n])
		fmt.Printf("SOCKET_ECHO_UDP_RECEIVED=%s body=%s\n", addr.String(), body)
		_, _ = conn.WriteToUDP([]byte("echo:"+body), addr)
	}
}

func runTCP(port int, stop <-chan struct{}) {
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcp listen failed: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Printf("SOCKET_ECHO_TCP_READY=%d\n", port)
	for {
		select {
		case <-stop:
			return
		default:
		}
		if tcp, ok := listener.(*net.TCPListener); ok {
			_ = tcp.SetDeadline(deadline())
		}
		conn, err := listener.Accept()
		if timeout(err) {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcp accept failed: %v\n", err)
			continue
		}
		go handleTCP(conn)
	}
}

func handleTCP(conn net.Conn) {
	defer conn.Close()
	payload, err := io.ReadAll(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcp read failed: %v\n", err)
		return
	}
	body := string(payload)
	fmt.Printf("SOCKET_ECHO_TCP_RECEIVED=%s body=%s\n", conn.RemoteAddr().String(), body)
	_, _ = conn.Write([]byte("echo:" + body))
}

func deadline() time.Time {
	return time.Now().Add(500 * time.Millisecond)
}

func timeout(err error) bool {
	if err == nil {
		return false
	}
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
