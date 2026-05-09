package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	var address string
	var body string
	var timeout time.Duration
	flag.StringVar(&address, "address", "", "target UDP address, for example 100.64.0.11:19090")
	flag.StringVar(&body, "body", "slan-socket-smoke", "UDP payload")
	flag.DurationVar(&timeout, "timeout", 8*time.Second, "UDP read/write timeout")
	flag.Parse()
	address = strings.TrimSpace(address)
	body = strings.TrimSpace(body)
	if address == "" || body == "" {
		fail("address and body are required")
	}
	target, err := net.ResolveUDPAddr("udp", address)
	must(err)
	conn, err := net.ListenUDP("udp", nil)
	must(err)
	defer conn.Close()
	must(conn.SetDeadline(time.Now().Add(timeout)))
	_, err = conn.WriteToUDP([]byte(body), target)
	must(err)
	buffer := make([]byte, 64*1024)
	n, from, err := conn.ReadFromUDP(buffer)
	must(err)
	got := string(buffer[:n])
	want := "echo:" + body
	if got != want {
		fail("unexpected UDP echo from %s: got=%q want=%q", from.String(), got, want)
	}
	fmt.Printf("udpEchoClient: ok target=%s response=%q\n", address, got)
}

func must(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
