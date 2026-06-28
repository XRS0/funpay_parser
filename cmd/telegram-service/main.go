package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"funpay-parser/internal/config"
	"funpay-parser/internal/events"
	"funpay-parser/internal/telegram"
)

func main() {
	cfg := config.Load()
	brokers := events.Brokers(cfg.KafkaBrokers)
	if len(brokers) == 0 {
		log.Fatal("KAFKA_BROKERS is required for telegram-service")
	}
	group := os.Getenv("TELEGRAM_KAFKA_GROUP")
	if group == "" {
		group = "telegram-service"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Println("telegram-service consuming", cfg.KafkaDealTopic, "from", cfg.KafkaBrokers)
	err := events.ConsumeDeals(ctx, brokers, cfg.KafkaDealTopic, group, func(ctx context.Context, ev events.DealFoundEvent) error {
		chatID := cfg.EffectiveTelegramChatID()
		token := cfg.EffectiveTelegramBotToken()
		if token == "" || chatID == "" {
			log.Println("telegram skipped: token/chat id not configured")
			return nil
		}
		text := telegram.DealMessage(ev.Result)
		if text == "" {
			return nil
		}
		mctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := telegram.NewWithProxy(token, cfg.EffectiveTelegramProxyURL()).SendDealReport(mctx, chatID, ev.Result); err != nil {
			log.Println("telegram notification failed:", err)
			return nil
		}
		log.Println("telegram notification sent from Kafka event")
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
