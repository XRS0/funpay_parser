package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
	"time"

	"funpay-parser/internal/models"
	"funpay-parser/internal/runner"
)

type Client struct {
	Token string
	http  *http.Client
}

type BotInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"first_name"`
}

func New(token string) *Client {
	return NewWithProxy(token, "")
}

func NewWithProxy(token string, proxyURL string) *Client {
	var baseDialer proxy.Dialer = proxy.Direct
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			if d, err := proxy.FromURL(u, proxy.Direct); err == nil {
				baseDialer = d
			}
		}
	}
	transport := &http.Transport{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return baseDialer.Dial("tcp", address)
		}
		if proxyURL != "" {
			return baseDialer.Dial("tcp", net.JoinHostPort(host, port))
		}
		if host == "api.telegram.org" {
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
			if err == nil {
				for _, ip := range ips {
					conn, dialErr := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), port))
					if dialErr == nil {
						return conn, nil
					}
					err = dialErr
				}
			}
			return nil, err
		}
		return (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", address)
	}
	return &Client{Token: strings.TrimSpace(token), http: &http.Client{Timeout: 15 * time.Second, Transport: transport}}
}

func (c *Client) enabled() bool { return c != nil && c.Token != "" }
func (c *Client) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.Token, method)
}

func (c *Client) safeErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if c.Token != "" {
		msg = strings.ReplaceAll(msg, c.Token, "***")
	}
	return fmt.Errorf("%s", msg)
}

func (c *Client) GetMe(ctx context.Context) (BotInfo, error) {
	if !c.enabled() {
		return BotInfo{}, fmt.Errorf("telegram bot token is empty")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("getMe"), nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return BotInfo{}, c.safeErr(err)
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool    `json:"ok"`
		Result      BotInfo `json:"result"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return BotInfo{}, err
	}
	if !out.OK {
		if out.Description == "" {
			out.Description = fmt.Sprintf("telegram getMe failed with http %d", resp.StatusCode)
		}
		return BotInfo{}, fmt.Errorf("%s", out.Description)
	}
	return out.Result, nil
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (c *Client) LatestChat(ctx context.Context) (Chat, error) {
	if !c.enabled() {
		return Chat{}, fmt.Errorf("telegram bot token is empty")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("getUpdates?limit=20&timeout=0"), nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return Chat{}, c.safeErr(err)
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message *struct {
				Chat Chat   `json:"chat"`
				Text string `json:"text"`
			} `json:"message"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Chat{}, err
	}
	if !out.OK {
		if out.Description == "" {
			out.Description = fmt.Sprintf("telegram getUpdates failed with http %d", resp.StatusCode)
		}
		return Chat{}, fmt.Errorf("%s", out.Description)
	}
	for i := len(out.Result) - 1; i >= 0; i-- {
		if out.Result[i].Message != nil && out.Result[i].Message.Chat.ID != 0 {
			return out.Result[i].Message.Chat, nil
		}
	}
	return Chat{}, fmt.Errorf("no Telegram chats found yet; open the bot and send /start, then try again")
}

func (c *Client) FindStartCode(ctx context.Context, code string) (Chat, error) {
	if !c.enabled() {
		return Chat{}, fmt.Errorf("telegram bot token is empty")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Chat{}, fmt.Errorf("telegram link code is empty")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("getUpdates?limit=100&timeout=0"), nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return Chat{}, c.safeErr(err)
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message *struct {
				Chat Chat   `json:"chat"`
				Text string `json:"text"`
			} `json:"message"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Chat{}, err
	}
	if !out.OK {
		if out.Description == "" {
			out.Description = fmt.Sprintf("telegram getUpdates failed with http %d", resp.StatusCode)
		}
		return Chat{}, fmt.Errorf("%s", out.Description)
	}
	wantedA := "/start " + code
	wantedB := "/start_" + code
	for i := len(out.Result) - 1; i >= 0; i-- {
		msg := out.Result[i].Message
		if msg == nil || msg.Chat.ID == 0 {
			continue
		}
		text := strings.TrimSpace(msg.Text)
		if text == wantedA || text == wantedB || text == code {
			return msg.Chat, nil
		}
	}
	return Chat{}, fmt.Errorf("open the bot and send /start %s, then press confirm", code)
}

func (c *Client) SendMessage(ctx context.Context, chatID string, text string) error {
	if !c.enabled() {
		return fmt.Errorf("telegram bot token is empty")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("telegram chat id is empty")
	}
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": false,
		"disable_notification":     false,
		"protect_content":          false,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL("sendMessage"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return c.safeErr(err)
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if !out.OK {
		if out.Description == "" {
			out.Description = fmt.Sprintf("telegram sendMessage failed with http %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", out.Description)
	}
	return nil
}

func (c *Client) SendPhoto(ctx context.Context, chatID string, pngBytes []byte, caption string) error {
	if !c.enabled() {
		return fmt.Errorf("telegram bot token is empty")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("telegram chat id is empty")
	}
	if len(pngBytes) == 0 {
		return fmt.Errorf("telegram photo is empty")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("chat_id", chatID)
	_ = mw.WriteField("caption", caption)
	_ = mw.WriteField("parse_mode", "HTML")
	_ = mw.WriteField("disable_notification", "false")
	part, err := mw.CreateFormFile("photo", "funpay-report.png")
	if err != nil {
		return err
	}
	if _, err := part.Write(pngBytes); err != nil {
		return err
	}
	_ = mw.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL("sendPhoto"), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return c.safeErr(err)
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if !out.OK {
		if out.Description == "" {
			out.Description = fmt.Sprintf("telegram sendPhoto failed with http %d", resp.StatusCode)
		}
		return fmt.Errorf("%s", out.Description)
	}
	return nil
}

func (c *Client) SendDealReport(ctx context.Context, chatID string, res runner.Result) error {
	text := DealMessage(res)
	if text == "" {
		return nil
	}
	img, err := DealReportImage(res)
	if err == nil {
		if err := c.SendPhoto(ctx, chatID, img, DealCaption(res)); err == nil {
			return nil
		}
	}
	return c.SendMessage(ctx, chatID, text)
}

func DealCaption(res runner.Result) string {
	if res.Cheapest == nil {
		return ""
	}
	l := res.Cheapest
	s := res.Summary
	reason := strings.TrimSpace(l.ClassificationReason)
	if reason == "" {
		reason = "подтверждён как личный аккаунт"
	}
	return fmt.Sprintf(
		"🌌 <b>Funpay Parser — персональный отчёт готов</b>\n"+
			"Данные отправлены только в Telegram, привязанный к аккаунту.\n\n"+
			"🏆 <b>%s</b>\n"+
			"💰 <b>%.2f %s</b> · 👤 %s · 🎯 %s\n\n"+
			"📊 Лотов: <b>%d</b> · LLM: <b>%d</b> · Личных: <b>%d</b> · Общих: <b>%d</b>\n"+
			"🧠 %s\n\n"+
			"🔗 <a href=\"%s\">Открыть лот на Funpay</a>",
		html.EscapeString(truncate(l.Title, 115)),
		l.Price,
		html.EscapeString(l.Currency),
		html.EscapeString(truncate(emptyDash(l.Seller), 36)),
		confidence(l),
		s["total_plus"],
		s["classified"],
		s["personal"],
		s["shared"],
		html.EscapeString(truncate(reason, 180)),
		html.EscapeString(l.URL),
	)
}

func telegramCaption(text string) string {
	r := []rune(text)
	if len(r) <= 950 {
		return text
	}
	return string(r[:930]) + "…"
}

func DealMessage(res runner.Result) string {
	if res.Cheapest == nil {
		return ""
	}
	l := res.Cheapest
	s := res.Summary
	reason := strings.TrimSpace(l.ClassificationReason)
	if reason == "" {
		reason = "LLM подтвердил, что это похоже на личный аккаунт."
	}
	return fmt.Sprintf(
		"🌌 <b>Funpay Parser — персональный отчёт готов</b>\n"+
			"Данные отправлены только в Telegram, привязанный к аккаунту.\n\n"+
			"🏆 <b>Самый дешёвый личный аккаунт</b>\n"+
			"<b>%s</b>\n\n"+
			"💰 <b>Цена:</b> %.2f %s\n"+
			"👤 <b>Продавец:</b> %s\n"+
			"🎯 <b>Confidence:</b> %s\n\n"+
			"📊 <b>Статистика</b>\n"+
			"• Лотов найдено: <b>%d</b>\n"+
			"• Проверено LLM: <b>%d</b>\n"+
			"• Личных: <b>%d</b>\n"+
			"• Общих: <b>%d</b>\n"+
			"• Прочее: <b>%d</b>\n\n"+
			"🧠 <b>Почему выбран:</b> %s\n\n"+
			"🔗 <a href=\"%s\">Открыть лот на Funpay</a>",
		html.EscapeString(l.Title),
		l.Price,
		html.EscapeString(l.Currency),
		html.EscapeString(l.Seller),
		confidence(l),
		s["total_plus"],
		s["classified"],
		s["personal"],
		s["shared"],
		s["other"],
		html.EscapeString(reason),
		html.EscapeString(l.URL),
	)
}

func confidence(l *models.Listing) string {
	if l.Confidence == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *l.Confidence)
}
