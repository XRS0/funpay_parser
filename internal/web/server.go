package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"funpay-parser/internal/config"
	"funpay-parser/internal/events"
	"funpay-parser/internal/rpc"
	"funpay-parser/internal/runner"
	"funpay-parser/internal/store"
	"funpay-parser/internal/telegram"
)

type RunState struct {
	Running    bool                `json:"running"`
	Status     string              `json:"status"`
	Progress   []map[string]string `json:"progress"`
	Result     *runner.Result      `json:"-"`
	Error      any                 `json:"error"`
	StartedAt  any                 `json:"started_at"`
	FinishedAt any                 `json:"finished_at"`
	ProfileID  any                 `json:"profile_id"`
}
type parserRunner interface {
	Run(ctx context.Context, opt runner.Options, progress func(string)) (runner.Result, error)
}

type Server struct {
	cfg           config.Config
	st            *store.Store
	mux           *http.ServeMux
	mu            sync.Mutex
	state         RunState
	cancel        context.CancelFunc
	parserRunner  parserRunner
	dealPublisher *events.Publisher
}

func New(cfg config.Config, st *store.Store) *Server {
	s := &Server{cfg: cfg, st: st, mux: http.NewServeMux(), state: RunState{Status: "idle", Progress: []map[string]string{}}}
	if cfg.ParserServiceAddr != "" {
		if pc, err := rpc.DialParser(cfg.ParserServiceAddr); err == nil {
			s.parserRunner = pc
			log.Println("api: using parser-service over gRPC at", cfg.ParserServiceAddr)
		} else {
			log.Println("api: parser-service unavailable, using local runner:", err)
		}
	}
	if cfg.KafkaBrokers != "" {
		s.dealPublisher = events.NewPublisher(events.Brokers(cfg.KafkaBrokers), cfg.KafkaDealTopic)
		log.Println("api: Kafka deal publisher configured", cfg.KafkaBrokers, cfg.KafkaDealTopic)
	}
	s.routes()
	go s.schedulerLoop()
	return s
}
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) schedulerLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		schedules, err := s.st.ListSchedules(context.Background())
		if err != nil {
			continue
		}
		now := time.Now().UTC()
		for _, sc := range schedules {
			if !sc.Enabled || sc.NextRunAt == nil {
				continue
			}
			next, err := time.Parse(time.RFC3339, *sc.NextRunAt)
			if err != nil || next.After(now) {
				continue
			}
			profile, _ := s.st.GetProfile(context.Background(), sc.ProfileID)
			if profile == nil {
				continue
			}
			s.st.TouchScheduleRun(context.Background(), sc.ID, now.Add(time.Duration(sc.IntervalMinutes)*time.Minute))
			_ = s.startRun(profile.ID, profileOpt(*profile))
		}
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("/run", s.run)
	s.mux.HandleFunc("/status", s.status)
	s.mux.HandleFunc("/stop", s.stop)
	s.mux.HandleFunc("/results", s.results)
	s.mux.HandleFunc("/api/settings", s.settings)
	s.mux.HandleFunc("/api/telegram/sync", s.telegramSync)
	s.mux.HandleFunc("/api/telegram/test", s.telegramTest)
	s.mux.HandleFunc("/api/profiles", s.profiles)
	s.mux.HandleFunc("/api/profiles/", s.profileByID)
	s.mux.HandleFunc("/api/saved_results", s.saved)
	s.mux.HandleFunc("/api/saved_results/", s.savedByID)
	s.mux.HandleFunc("/api/schedules", s.schedules)
	s.mux.HandleFunc("/api/schedules/", s.scheduleByID)
	s.mux.HandleFunc("/", s.spa)
}

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/run" || r.URL.Path == "/status" || r.URL.Path == "/stop" || r.URL.Path == "/results" {
		http.NotFound(w, r)
		return
	}
	dist := "frontend/dist"
	path := filepath.Join(dist, filepath.Clean(r.URL.Path))
	if r.URL.Path != "/" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
	}
	http.ServeFile(w, r, filepath.Join(dist, "index.html"))
}
func jsonOut(w http.ResponseWriter, v any, code ...int) {
	w.Header().Set("Content-Type", "application/json")
	c := 200
	if len(code) > 0 {
		c = code[0]
	}
	w.WriteHeader(c)
	_ = json.NewEncoder(w).Encode(v)
}
func decode(r *http.Request, v any) { _ = json.NewDecoder(r.Body).Decode(v) }

func (s *Server) progress(msg string) {
	log.Println(msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Status = msg
	s.state.Progress = append(s.state.Progress, map[string]string{"time": time.Now().Format("15:04:05"), "message": msg})
}
func (s *Server) startRun(profileID any, opt runner.Options) error {
	s.mu.Lock()
	if s.state.Running {
		s.mu.Unlock()
		return fmt.Errorf("A parse task is already running.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.state = RunState{Running: true, Status: "Starting parser...", Progress: []map[string]string{}, StartedAt: time.Now().Format("15:04:05"), ProfileID: profileID}
	s.mu.Unlock()
	go func() {
		var res runner.Result
		var err error
		if s.parserRunner != nil {
			res, err = s.parserRunner.Run(ctx, opt, s.progress)
		} else {
			res, err = runner.Run(ctx, s.cfg, opt, s.progress)
		}
		s.mu.Lock()
		if err != nil {
			if errors.As(err, &runner.Cancelled{}) {
				s.state.Status = "Остановлено"
			} else {
				s.state.Status = "Error: " + err.Error()
				s.state.Error = err.Error()
			}
		} else {
			s.state.Result = &res
			if res.Success {
				s.state.Status = "Done"
			} else {
				s.state.Status = "Failed"
			}
		}
		s.state.Running = false
		s.state.FinishedAt = time.Now().Format("15:04:05")
		s.cancel = nil
		s.mu.Unlock()

		if err == nil && res.Success && profileID != nil {
			if id, ok := profileID.(int); ok && id > 0 {
				_, _ = s.st.SaveResult(context.Background(), id, res.Cheapest, res.Summary, res.AllResults)
			}
		}
		if err == nil && res.Success && res.Cheapest != nil {
			s.notifyTelegram(res)
		}
	}()
	return nil
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var d map[string]any
	decode(r, &d)
	var opt runner.Options
	var pid any = nil
	if v := toInt(d["profile_id"]); v > 0 {
		p, _ := s.st.GetProfile(r.Context(), v)
		if p == nil {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		pid = p.ID
		opt = profileOpt(*p)
	} else {
		opt = runner.Options{CategoryID: orInt(d["category_id"], 1355), Query: orStr(d["query"], "chatgpt plus"), Candidates: orInt(d["candidates"], 40), Pages: toInt(d["pages"]), Deep: toBool(d["deep"]), ScrapeWorkers: orInt(d["scrape_workers"], 4), LLMWorkers: orInt(d["llm_workers"], 8)}
	}
	if err := s.startRun(pid, opt); err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 409)
		return
	}
	jsonOut(w, map[string]any{"success": true, "status": "started"})
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]any{"running": s.state.Running, "status": s.state.Status, "progress": s.state.Progress, "started_at": s.state.StartedAt, "finished_at": s.state.FinishedAt, "error": s.state.Error, "profile_id": s.state.ProfileID}
	if s.state.Result != nil {
		out["result_summary"] = s.state.Result.Summary
		out["cheapest"] = s.state.Result.Cheapest
	} else {
		out["result_summary"] = nil
		out["cheapest"] = nil
	}
	jsonOut(w, out)
}
func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Running || s.cancel == nil {
		jsonOut(w, map[string]string{"error": "No parse task is running."}, 409)
		return
	}
	s.cancel()
	s.state.Status = "Останавливаю..."
	jsonOut(w, map[string]any{"success": true, "status": "stopping"})
}
func (s *Server) results(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Result == nil {
		jsonOut(w, map[string]string{"error": "No results yet"}, 404)
		return
	}
	jsonOut(w, s.state.Result)
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	respond := func() {
		key := s.cfg.EffectiveAPIKey()
		set := config.LoadSettings(s.cfg.SettingsFile)
		tgToken := s.cfg.EffectiveTelegramBotToken()
		botUsername := ""
		if tgToken != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			if bot, err := telegram.NewWithProxy(tgToken, s.cfg.EffectiveTelegramProxyURL()).GetMe(ctx); err == nil {
				botUsername = bot.Username
			}
			cancel()
		}
		jsonOut(w, map[string]any{
			"llm_provider":           s.cfg.EffectiveProvider(),
			"llm_model":              s.cfg.EffectiveModel(),
			"has_key":                key != "",
			"llm_api_key":            maskSecret(key),
			"telegram_has_token":     tgToken != "",
			"telegram_bot_token":     maskSecret(tgToken),
			"telegram_chat_id":       set.TelegramChatID,
			"telegram_bot_username":  botUsername,
			"telegram_proxy":         set.TelegramProxy,
			"telegram_proxy_active":  s.cfg.EffectiveTelegramProxyURL() != "",
			"telegram_notifications": tgToken != "" && set.TelegramChatID != "",
		})
	}
	switch r.Method {
	case http.MethodGet:
		respond()
	case http.MethodPost, http.MethodPut:
		var d map[string]any
		decode(r, &d)
		set := config.LoadSettings(s.cfg.SettingsFile)
		if p := orStr(d["llm_provider"], orStr(d["provider"], "")); p != "" {
			set.LLMProvider = p
		}
		if _, ok := d["llm_api_key"]; ok {
			set.LLMAPIKey = orStr(d["llm_api_key"], "")
		} else if k := orStr(d["api_key"], ""); k != "" {
			set.LLMAPIKey = k
		}
		if m := orStr(d["llm_model"], orStr(d["model"], "")); m != "" {
			set.LLMModel = m
		}
		if _, ok := d["telegram_bot_token"]; ok {
			set.TelegramBotToken = orStr(d["telegram_bot_token"], "")
		}
		if _, ok := d["telegram_chat_id"]; ok {
			set.TelegramChatID = orStr(d["telegram_chat_id"], "")
		}
		if _, ok := d["telegram_proxy"]; ok {
			set.TelegramProxy = orStr(d["telegram_proxy"], "")
		}
		_ = config.SaveSettings(s.cfg.SettingsFile, set)
		respond()
	default:
		http.NotFound(w, r)
	}
}

func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) > 8 {
		return v[:4] + "..." + v[len(v)-4:]
	}
	return "***"
}

func (s *Server) telegramSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	set := config.LoadSettings(s.cfg.SettingsFile)
	token := s.cfg.EffectiveTelegramBotToken()
	client := telegram.NewWithProxy(token, s.cfg.EffectiveTelegramProxyURL())
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	bot, err := client.GetMe(ctx)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	chat, err := client.LatestChat(ctx)
	if err != nil {
		jsonOut(w, map[string]any{"error": err.Error(), "bot_username": bot.Username}, 400)
		return
	}
	set.TelegramBotToken = token
	set.TelegramChatID = strconv.FormatInt(chat.ID, 10)
	_ = config.SaveSettings(s.cfg.SettingsFile, set)
	jsonOut(w, map[string]any{"success": true, "bot_username": bot.Username, "chat_id": set.TelegramChatID, "chat_username": chat.Username, "chat_first_name": chat.FirstName})
}

func (s *Server) telegramTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	chatID := s.cfg.EffectiveTelegramChatID()
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	err := telegram.NewWithProxy(s.cfg.EffectiveTelegramBotToken(), s.cfg.EffectiveTelegramProxyURL()).SendMessage(ctx, chatID, "✅ Funpay Parser: тестовое уведомление работает.")
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	jsonOut(w, map[string]any{"success": true})
}

func (s *Server) notifyTelegram(res runner.Result) {
	if s.dealPublisher != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.dealPublisher.PublishDealFound(ctx, events.DealFoundEvent{Result: res, CreatedAt: time.Now().UTC()}); err != nil {
			log.Println("kafka deal publish failed, falling back to direct telegram:", err)
		} else {
			log.Println("deal event published to Kafka")
			return
		}
	}
	token := s.cfg.EffectiveTelegramBotToken()
	chatID := s.cfg.EffectiveTelegramChatID()
	if token == "" || chatID == "" {
		log.Println("telegram notification skipped: token or chat id is not configured")
		return
	}
	text := telegram.DealMessage(res)
	if text == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := telegram.NewWithProxy(token, s.cfg.EffectiveTelegramProxyURL()).SendMessage(ctx, chatID, text); err != nil {
		log.Println("telegram notification failed:", err)
		return
	}
	log.Println("telegram notification sent")
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, _ := s.st.ListProfiles(r.Context())
		jsonOut(w, x)
	case http.MethodPost:
		var p store.Profile
		decode(r, &p)
		if p.Name == "" {
			jsonOut(w, map[string]string{"error": "Name is required"}, 400)
			return
		}
		if p.Query == "" {
			p.Query = "chatgpt plus"
		}
		if p.CategoryID == 0 {
			p.CategoryID = 1355
		}
		if p.Candidates == 0 {
			p.Candidates = 40
		}
		x, err := s.st.CreateProfile(r.Context(), p)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		jsonOut(w, x, 201)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) profileByID(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path, "/api/profiles/")
	id := atoi(parts[0])
	if len(parts) > 1 && parts[1] == "run" && r.Method == http.MethodPost {
		p, _ := s.st.GetProfile(r.Context(), id)
		if p == nil {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		if err := s.startRun(p.ID, profileOpt(*p)); err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 409)
			return
		}
		jsonOut(w, map[string]any{"success": true, "status": "started"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, _ := s.st.GetProfile(r.Context(), id)
		if p == nil {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		jsonOut(w, p)
	case http.MethodPut:
		var p store.Profile
		decode(r, &p)
		x, err := s.st.UpdateProfile(r.Context(), id, p)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		if x == nil {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		jsonOut(w, x)
	case http.MethodDelete:
		ok, _ := s.st.DeleteProfile(r.Context(), id)
		if !ok {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		jsonOut(w, map[string]bool{"success": true})
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) saved(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pid, _ := strconv.Atoi(r.URL.Query().Get("profile_id"))
		x, _ := s.st.ListSaved(r.Context(), pid)
		jsonOut(w, x)
	case http.MethodPost:
		var d map[string]any
		decode(r, &d)
		pid := toInt(d["profile_id"])
		x, err := s.st.SaveResult(r.Context(), pid, d["cheapest"], d["summary"], d["all_results"])
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		jsonOut(w, x, 201)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) savedByID(w http.ResponseWriter, r *http.Request) {
	id := atoi(strings.TrimPrefix(r.URL.Path, "/api/saved_results/"))
	if r.Method == http.MethodGet {
		x, _ := s.st.GetSaved(r.Context(), id)
		if x == nil {
			jsonOut(w, map[string]string{"error": "Saved result not found"}, 404)
			return
		}
		jsonOut(w, x)
		return
	}
	if r.Method == http.MethodDelete {
		ok, _ := s.st.DeleteSaved(r.Context(), id)
		if !ok {
			jsonOut(w, map[string]string{"error": "Saved result not found"}, 404)
			return
		}
		jsonOut(w, map[string]bool{"success": true})
		return
	}
	http.NotFound(w, r)
}
func (s *Server) schedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		x, _ := s.st.ListSchedules(r.Context())
		jsonOut(w, x)
	case http.MethodPost:
		var sc store.Schedule
		decode(r, &sc)
		if sc.IntervalMinutes < 1 {
			jsonOut(w, map[string]string{"error": "interval_minutes must be a positive integer"}, 400)
			return
		}
		x, err := s.st.CreateSchedule(r.Context(), sc)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		jsonOut(w, x, 201)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) scheduleByID(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path, "/api/schedules/")
	id := atoi(parts[0])
	if len(parts) > 1 && parts[1] == "run" && r.Method == http.MethodPost {
		sc, _ := s.st.GetSchedule(r.Context(), id)
		if sc == nil {
			jsonOut(w, map[string]string{"error": "Schedule not found"}, 404)
			return
		}
		p, _ := s.st.GetProfile(r.Context(), sc.ProfileID)
		if p == nil {
			jsonOut(w, map[string]string{"error": "Profile not found"}, 404)
			return
		}
		if err := s.startRun(p.ID, profileOpt(*p)); err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 409)
			return
		}
		jsonOut(w, map[string]any{"success": true, "status": "started"})
		return
	}
	if r.Method == http.MethodPut {
		var sc store.Schedule
		decode(r, &sc)
		x, err := s.st.UpdateSchedule(r.Context(), id, sc)
		if err != nil {
			jsonOut(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		jsonOut(w, x)
		return
	}
	if r.Method == http.MethodDelete {
		ok, _ := s.st.DeleteSchedule(r.Context(), id)
		if !ok {
			jsonOut(w, map[string]string{"error": "Schedule not found"}, 404)
			return
		}
		jsonOut(w, map[string]bool{"success": true})
		return
	}
	http.NotFound(w, r)
}

func profileOpt(p store.Profile) runner.Options {
	pages := 0
	if p.MaxPages != nil {
		pages = *p.MaxPages
	}
	return runner.Options{CategoryID: p.CategoryID, Query: p.Query, Candidates: p.Candidates, MaxPages: pages, Deep: p.Deep}
}
func splitPath(p, prefix string) []string {
	return strings.Split(strings.Trim(strings.TrimPrefix(p, prefix), "/"), "/")
}
func atoi(s string) int { i, _ := strconv.Atoi(s); return i }
func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		i, _ := strconv.Atoi(x)
		return i
	default:
		return 0
	}
}
func orInt(v any, d int) int {
	if i := toInt(v); i != 0 {
		return i
	}
	return d
}
func orStr(v any, d string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return d
}
func toBool(v any) bool               { b, ok := v.(bool); return ok && b }
func EnsureDataDir(cfg config.Config) { _ = os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755) }
