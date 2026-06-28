package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"funpay-parser/internal/config"
	"funpay-parser/internal/models"
)

type Client struct {
	cfg  config.Config
	http *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}}
}

type Result struct {
	IsPlus      bool    `json:"is_plus"`
	AccountType string  `json:"account_type"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
}

func (c *Client) Classify(ctx context.Context, l models.Listing) (Result, error) {
	provider, key, model := c.cfg.EffectiveProvider(), c.cfg.EffectiveAPIKey(), c.cfg.EffectiveModel()
	if key == "" {
		return Result{}, fmt.Errorf("%s API key is not configured. Set it in Settings", strings.Title(provider))
	}
	url := "https://api.fireworks.ai/inference/v1/chat/completions"
	headers := map[string]string{"Authorization": "Bearer " + key, "Content-Type": "application/json"}
	if provider == "openrouter" {
		url = "https://openrouter.ai/api/v1/chat/completions"
		headers["HTTP-Referer"] = "https://github.com/XRS0/funpay_parser"
		headers["X-Title"] = "Funpay Parser"
	}
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": "You are a helpful classifier that outputs only JSON."}, {"role": "user", "content": prompt(l)}}, "temperature": 0.1, "max_tokens": 256, "response_format": map[string]string{"type": "json_object"}}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("llm http %d", resp.StatusCode)
	}
	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Result{}, err
	}
	if len(data.Choices) == 0 {
		return Result{}, fmt.Errorf("empty llm response")
	}
	var r Result
	if err := json.Unmarshal([]byte(data.Choices[0].Message.Content), &r); err != nil {
		return Result{}, err
	}
	r.AccountType = strings.ToLower(strings.TrimSpace(r.AccountType))
	if r.AccountType == "" {
		r.AccountType = "unknown"
	}
	return r, nil
}

func (c *Client) ClassifyMany(ctx context.Context, listings []models.Listing, workers int, progress func(string)) []models.Listing {
	if workers <= 0 {
		workers = 8
	}
	jobs := make(chan int)
	out := make(chan models.Listing, len(listings))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				l := listings[i]
				r, err := c.Classify(ctx, l)
				if err != nil {
					unk := "unknown"
					zero := 0.0
					l.AccountType = &unk
					l.Confidence = &zero
					l.ClassificationReason = "classification error: " + err.Error()
				} else {
					l.IsPlus = &r.IsPlus
					l.AccountType = &r.AccountType
					l.Confidence = &r.Confidence
					l.ClassificationReason = r.Reason
				}
				if progress != nil {
					progress(fmt.Sprintf("[LLM] %s | type=%s", trim(l.Title, 50), value(l.AccountType)))
				}
				out <- l
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range listings {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()
	res := []models.Listing{}
	for l := range out {
		res = append(res, l)
	}
	return res
}
func prompt(l models.Listing) string {
	return fmt.Sprintf("You are analyzing a Funpay marketplace listing for a ChatGPT account.\n\nTitle: %s\nDescription: %s\nPrice: %.2f %s\n\nAnswer: is ChatGPT Plus? account_type personal/shared/unknown? confidence 0.0-1.0. Respond ONLY valid JSON: {\"is_plus\": true|false, \"account_type\": \"personal\"|\"shared\"|\"unknown\", \"confidence\": 0.0, \"reason\": \"...\"}", l.Title, l.Description, l.Price, l.Currency)
}
func trim(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
func value(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
