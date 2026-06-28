package events

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"funpay-parser/internal/runner"
	"github.com/segmentio/kafka-go"
)

type DealFoundEvent struct {
	RunID     string        `json:"run_id,omitempty"`
	Result    runner.Result `json:"result"`
	CreatedAt time.Time     `json:"created_at"`
}

func Brokers(csv string) []string {
	parts := strings.Split(csv, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers []string, topic string) *Publisher {
	if len(brokers) == 0 || topic == "" {
		return nil
	}
	return &Publisher{writer: &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: topic, Balancer: &kafka.LeastBytes{}, RequiredAcks: kafka.RequireOne, Async: false, AllowAutoTopicCreation: true}}
}

func (p *Publisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
func (p *Publisher) PublishDealFound(ctx context.Context, event DealFoundEvent) error {
	if p == nil || p.writer == nil {
		return nil
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(event.RunID), Value: b, Time: event.CreatedAt})
}

type DealHandler func(context.Context, DealFoundEvent) error

func ConsumeDeals(ctx context.Context, brokers []string, topic, groupID string, handler DealHandler) error {
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, GroupID: groupID, MinBytes: 1, MaxBytes: 10e6, CommitInterval: time.Second})
	defer r.Close()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			return err
		}
		var ev DealFoundEvent
		if err := json.Unmarshal(m.Value, &ev); err == nil && handler != nil {
			_ = handler(ctx, ev)
		}
		_ = r.CommitMessages(ctx, m)
	}
}
