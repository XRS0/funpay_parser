package main

import (
	"log"
	"net/http"
	"os"

	"funpay-parser/internal/auth/handlers"
	"funpay-parser/internal/auth/jwt"
	authstore "funpay-parser/internal/auth/store"
	"funpay-parser/internal/config"
)

func main() {
	cfg := config.Load()
	_ = os.MkdirAll(cfg.DataDir, 0755)
	jwtSecret := cfg.AuthJWTSecret
	if jwtSecret == "" {
		log.Fatal("AUTH_JWT_SECRET is required")
	}
	dbPath := cfg.AuthDatabasePath
	st, err := authstore.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	j := jwt.NewManager(jwtSecret)
	addr := os.Getenv("AUTH_HTTP_ADDR")
	if addr == "" {
		addr = ":5001"
	}
	botToken := cfg.EffectiveTelegramBotToken()
	cookieHost := cfg.AuthCookieHost
	h := handlers.New(handlers.Config{
		Store:      st,
		JWTManager: j,
		BotToken:   botToken,
		CookieHost: cookieHost,
	})
	log.Println("auth-service listening on", addr)
	log.Fatal(http.ListenAndServe(addr, h.Routes()))
}
