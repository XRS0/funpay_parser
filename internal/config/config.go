package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	DataDir           string
	SettingsFile      string
	RunStateFile      string
	DatabasePath      string
	LLMProvider       string
	FireworksKey      string
	FireworksModel    string
	OpenRouterKey     string
	OpenRouterModel   string
	FunpayBaseURL     string
	FunpayLang        string
	MaxPages          int
	Proxy             string
	ProxyURL          string
	TelegramProxy     string
	TelegramProxyURL  string
	KafkaBrokers      string
	KafkaDealTopic    string
	ParserServiceAddr string
	LLMServiceAddr    string
	Port              string
}

var mu sync.Mutex

func Load() Config {
	_ = godotenv.Load(".env")
	dataDir := getenv("DATA_DIR", ".")
	_ = os.MkdirAll(dataDir, 0755)
	maxPages, _ := strconv.Atoi(getenv("MAX_PAGES", "3"))
	if maxPages <= 0 {
		maxPages = 3
	}
	proxy := os.Getenv("PROXY")
	return Config{
		DataDir:           dataDir,
		SettingsFile:      filepath.Join(dataDir, "settings.json"),
		RunStateFile:      filepath.Join(dataDir, "run_state.json"),
		DatabasePath:      getenv("DATABASE_PATH", "parser.db"),
		LLMProvider:       getenv("LLM_PROVIDER", "fireworks"),
		FireworksKey:      os.Getenv("FIREWORKS_API_KEY"),
		FireworksModel:    getenv("FIREWORKS_MODEL", "accounts/fireworks/models/llama-v3p1-70b-instruct"),
		OpenRouterKey:     os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:   getenv("OPENROUTER_MODEL", "openai/gpt-4o-mini"),
		FunpayBaseURL:     strings.TrimRight(getenv("FUNPAY_BASE_URL", "https://funpay.com"), "/"),
		FunpayLang:        getenv("FUNPAY_LANG", "en"),
		MaxPages:          maxPages,
		Proxy:             proxy,
		ProxyURL:          ParseProxy(proxy),
		TelegramProxy:     getenv("TELEGRAM_PROXY", ""),
		TelegramProxyURL:  NormalizeProxyURL(getenv("TELEGRAM_PROXY", "")),
		KafkaBrokers:      getenv("KAFKA_BROKERS", ""),
		KafkaDealTopic:    getenv("KAFKA_DEAL_TOPIC", "deals.found"),
		ParserServiceAddr: getenv("PARSER_SERVICE_ADDR", ""),
		LLMServiceAddr:    getenv("LLM_SERVICE_ADDR", ""),
		Port:              getenv("PORT", "5000"),
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func ParseProxy(s string) string {
	if s == "" || !strings.Contains(s, "@") {
		return NormalizeProxyURL(s)
	}
	parts := strings.SplitN(s, "@", 2)
	hostPort, creds := parts[0], parts[1]
	cp := strings.SplitN(creds, ":", 2)
	if len(cp) != 2 {
		return ""
	}
	if _, _, ok := strings.Cut(hostPort, ":"); !ok {
		return ""
	}
	return fmt.Sprintf("socks5://%s:%s@%s", url.QueryEscape(cp[0]), url.QueryEscape(cp[1]), hostPort)
}

func NormalizeProxyURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		return s
	}
	if strings.Contains(s, ":") {
		return "socks5://" + s
	}
	return ""
}

type Settings struct {
	LLMProvider      string `json:"llm_provider,omitempty"`
	LLMAPIKey        string `json:"llm_api_key,omitempty"`
	LLMModel         string `json:"llm_model,omitempty"`
	TelegramBotToken string `json:"telegram_bot_token,omitempty"`
	TelegramChatID   string `json:"telegram_chat_id,omitempty"`
	TelegramProxy    string `json:"telegram_proxy,omitempty"`
	FunpayProxy      string `json:"funpay_proxy,omitempty"`
}

func LoadSettings(path string) Settings {
	mu.Lock()
	defer mu.Unlock()
	b, err := os.ReadFile(path)
	if err != nil {
		return Settings{}
	}
	var s Settings
	_ = json.Unmarshal(b, &s)
	return s
}

func SaveSettings(path string, s Settings) error {
	mu.Lock()
	defer mu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, b, 0600)
}

func (c Config) EffectiveProvider() string {
	s := LoadSettings(c.SettingsFile)
	if s.LLMProvider == "fireworks" || s.LLMProvider == "openrouter" {
		return s.LLMProvider
	}
	if c.LLMProvider == "openrouter" {
		return "openrouter"
	}
	return "fireworks"
}
func (c Config) EffectiveAPIKey() string {
	s := LoadSettings(c.SettingsFile)
	if s.LLMAPIKey != "" {
		return s.LLMAPIKey
	}
	if c.EffectiveProvider() == "openrouter" {
		return c.OpenRouterKey
	}
	return c.FireworksKey
}
func (c Config) EffectiveModel() string {
	s := LoadSettings(c.SettingsFile)
	if s.LLMModel != "" {
		return s.LLMModel
	}
	if c.EffectiveProvider() == "openrouter" {
		return c.OpenRouterModel
	}
	return c.FireworksModel
}

func (c Config) EffectiveTelegramBotToken() string {
	s := LoadSettings(c.SettingsFile)
	if s.TelegramBotToken != "" {
		return s.TelegramBotToken
	}
	return os.Getenv("TELEGRAM_BOT_TOKEN")
}

func (c Config) EffectiveTelegramChatID() string {
	s := LoadSettings(c.SettingsFile)
	if s.TelegramChatID != "" {
		return s.TelegramChatID
	}
	return os.Getenv("TELEGRAM_CHAT_ID")
}

func (c Config) EffectiveTelegramProxyURL() string {
	s := LoadSettings(c.SettingsFile)
	if s.TelegramProxy != "" {
		return NormalizeProxyURL(s.TelegramProxy)
	}
	return c.TelegramProxyURL
}

func (c Config) EffectiveFunpayProxy() string {
	s := LoadSettings(c.SettingsFile)
	if s.FunpayProxy != "" {
		return s.FunpayProxy
	}
	return c.Proxy
}

func (c Config) EffectiveFunpayProxyURL() string {
	return ParseProxy(c.EffectiveFunpayProxy())
}
