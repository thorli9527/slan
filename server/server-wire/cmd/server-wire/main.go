package main

import (
	"log"

	"github.com/slan/server/server-wire/internal/config"
	"github.com/slan/server/server-wire/internal/httpapi"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	if err := httpapi.ListenAndServe(cfg); err != nil {
		log.Fatal(err)
	}
}
