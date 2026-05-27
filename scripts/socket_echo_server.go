package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	var udpPort int
	var tcpPort int
	var listenHost string
	flag.IntVar(&udpPort, "udp-port", 19090, "UDP echo listen port")
	flag.IntVar(&tcpPort, "tcp-port", 19091, "TCP echo listen port")
	flag.StringVar(&listenHost, "listen-host", "0.0.0.0", "echo listen host")
	flag.Parse()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	if udpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runUDP(listenHost, udpPort, stop)
		}()
	}
	if tcpPort > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTCP(listenHost, tcpPort, stop)
		}()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stop)
	wg.Wait()
}

func runUDP(host string, port int, stop <-chan struct{}) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(host).To4(), Port: port})
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
		written, err := conn.WriteToUDP([]byte("echo:"+body), addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "udp write failed: %v\n", err)
			continue
		}
		fmt.Printf("SOCKET_ECHO_UDP_SENT=%s bytes=%d\n", addr.String(), written)
	}
}

func runTCP(host string, port int, stop <-chan struct{}) {
	listener, err := net.Listen("tcp4", fmt.Sprintf("%s:%d", host, port))
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
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	body, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcp read failed: %v\n", err)
		return
	}
	body = strings.TrimSuffix(body, "\n")
	fmt.Printf("SOCKET_ECHO_TCP_RECEIVED=%s body=%s\n", conn.RemoteAddr().String(), body)
	_, _ = conn.Write([]byte("echo:" + body + "\n"))
	time.Sleep(500 * time.Millisecond)
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
