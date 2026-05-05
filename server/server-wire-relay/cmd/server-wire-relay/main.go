package main

import (
	"log"

	"github.com/slan/server/server-wire-relay/internal/config"
	"github.com/slan/server/server-wire-relay/internal/relay"
)

func main() {
	cfg := config.Load()
	server, err := relay.NewUDPServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
