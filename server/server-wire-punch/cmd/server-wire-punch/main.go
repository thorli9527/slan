package main

import (
	"log"

	"github.com/slan/server/server-wire-punch/internal/config"
	"github.com/slan/server/server-wire-punch/internal/punch"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	server, err := punch.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
