package main

import (
	"fmt"
	"log"

	"github.com/nedaZarei/Cloud_vocabularyService/config"
	"github.com/nedaZarei/Cloud_vocabularyService/service"
)

func main() {
	cfg, err := config.InitConfig("/app/config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	fmt.Println(cfg)

	vocabularyService := service.NewService(cfg)

	if err := vocabularyService.StartService(); err != nil {
		log.Fatalf("failed to start service one: %v", err)
	}
}
