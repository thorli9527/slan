package main

import (
	"context"
	"log"

	"github.com/slan/service-biz/internal/bootstrap"
	"github.com/slan/service-biz/internal/repository"
)

func main() {
	store, err := repository.OpenGormStore(repository.GormConfig{})
	if err != nil {
		log.Fatal(err)
	}
	if err := bootstrap.InitializeGormStore(context.Background(), store); err != nil {
		log.Fatal(err)
	}
	if err := bootstrap.BackfillReferenceDefaults(context.Background(), store); err != nil {
		log.Fatal(err)
	}
	log.Println("service-biz defaults backfilled")
}
