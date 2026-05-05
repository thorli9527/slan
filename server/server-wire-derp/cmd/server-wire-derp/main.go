package main

import (
	"log"

	"github.com/slan/server/server-wire-derp/internal/config"
	"github.com/slan/server/server-wire-derp/internal/derp"
)

func main() {
	cfg := config.Load()
	server, err := derp.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
