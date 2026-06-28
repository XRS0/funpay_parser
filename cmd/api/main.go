package main

import (
	"log"
	"net/http"

	"funpay-parser/internal/config"
	"funpay-parser/internal/store"
	"funpay-parser/internal/web"
)

func main() {
	cfg := config.Load()
	web.EnsureDataDir(cfg)
	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	srv := web.New(cfg, st)
	log.Printf("api service listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, srv.Handler()))
}
