package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"funpay-parser/internal/auth/jwt"
	authstore "funpay-parser/internal/auth/store"
	"funpay-parser/internal/auth/telegramverify"
	"funpay-parser/internal/telegram"
)

type Handler struct {
	store      *authstore.Store
	jwt        *jwt.Manager
	botToken   string
	botProxy   string
	cookieHost string
}

type Config struct {
	Store      *authstore.Store
	JWTManager *jwt.Manager
	BotToken   string
	BotProxy   string
	CookieHost string
}

func New(cfg Config) *Handler {
	return &Handler{store: cfg.Store, jwt: cfg.JWTManager, botToken: cfg.BotToken, botProxy: cfg.BotProxy, cookieHost: cfg.CookieHost}
}

func jsonOut(w http.ResponseWriter, v any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) { _ = json.NewDecoder(r.Body).Decode(v) }

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func setRefreshCookie(w http.ResponseWriter, token, host string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
	}
	if host != "" && !strings.Contains(host, "localhost") {
		cookie.Domain = host
	}
	http.SetCookie(w, cookie)
}

func clearRefreshCookie(w http.ResponseWriter, host string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
	if host != "" && !strings.Contains(host, "localhost") {
		cookie.Domain = host
	}
	http.SetCookie(w, cookie)
}

func (h *Handler) issueTokens(ctx context.Context, user authstore.User) (string, string, error) {
	access, err := h.jwt.IssueAccess(user.ID, user.Email, user.Role, user.TelegramUserID)
	if err != nil {
		return "", "", err
	}
	refreshTokenID := generateID()
	refresh, err := h.jwt.IssueRefresh(user.ID, refreshTokenID)
	if err != nil {
		return "", "", err
	}
	exp, _ := h.jwt.Parse(refresh)
	var expires time.Time
	if exp != nil && exp.ExpiresAt != nil {
		expires = exp.ExpiresAt.Time
	} else {
		expires = time.Now().UTC().Add(7 * 24 * time.Hour)
	}
	if err := h.store.SaveRefreshToken(ctx, refreshTokenID, user.ID, hashToken(refresh), expires); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func generateID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func telegramCode() string { return strings.ToUpper(generateID()[:10]) }

func telegramDeepLink(botUsername, code string) string {
	botUsername = strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	if botUsername == "" || code == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s?start=%s", botUsername, code)
}

func telegramDisplayName(chat telegram.Chat) string {
	if chat.Username != "" {
		return chat.Username
	}
	return strings.TrimSpace(chat.FirstName + " " + chat.LastName)
}

func (h *Handler) ConfigInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	jsonOut(w, map[string]any{
		"auth_enabled":           true,
		"telegram_login_enabled": h.botToken != "",
	}, 200)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var d struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decode(r, &d)
	if strings.TrimSpace(d.Name) == "" || d.Email == "" || d.Password == "" || len(d.Password) < 6 {
		jsonOut(w, map[string]string{"error": "name, email and password (min 6 chars) are required"}, 400)
		return
	}
	user, err := h.store.CreateUser(r.Context(), d.Name, d.Email, d.Password)
	if err != nil {
		jsonOut(w, map[string]string{"error": "user already exists or registration failed"}, 409)
		return
	}
	access, refresh, err := h.issueTokens(r.Context(), user)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to issue tokens"}, 500)
		return
	}
	setRefreshCookie(w, refresh, h.cookieHost)
	jsonOut(w, map[string]any{"user": user, "access_token": access}, 201)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var d struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decode(r, &d)
	user, hash, err := h.store.GetUserByEmail(r.Context(), d.Email)
	if err != nil || !authstore.CheckPassword(hash, d.Password) {
		jsonOut(w, map[string]string{"error": "invalid email or password"}, 401)
		return
	}
	access, refresh, err := h.issueTokens(r.Context(), user)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to issue tokens"}, 500)
		return
	}
	setRefreshCookie(w, refresh, h.cookieHost)
	jsonOut(w, map[string]any{"user": user, "access_token": access}, 200)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	refresh := r.Header.Get("X-Refresh-Token")
	if refresh == "" {
		c, err := r.Cookie("refresh_token")
		if err == nil {
			refresh = c.Value
		}
	}
	if refresh == "" {
		jsonOut(w, map[string]string{"error": "refresh token is required"}, 401)
		return
	}
	claims, err := h.jwt.ValidateRefresh(refresh)
	if err != nil {
		jsonOut(w, map[string]string{"error": "invalid refresh token"}, 401)
		return
	}
	userID, tokenHash, revoked, expires, err := h.store.GetRefreshToken(r.Context(), claims.ID)
	if err != nil || revoked || time.Now().UTC().After(expires) || tokenHash != hashToken(refresh) {
		jsonOut(w, map[string]string{"error": "refresh token is revoked or expired"}, 401)
		return
	}
	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		jsonOut(w, map[string]string{"error": "user not found"}, 404)
		return
	}
	access, newRefresh, err := h.issueTokens(r.Context(), user)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to issue tokens"}, 500)
		return
	}
	_ = h.store.RevokeRefreshToken(r.Context(), claims.ID)
	setRefreshCookie(w, newRefresh, h.cookieHost)
	jsonOut(w, map[string]any{"user": user, "access_token": access}, 200)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	refresh := ""
	if c, err := r.Cookie("refresh_token"); err == nil {
		refresh = c.Value
	}
	if refresh == "" {
		refresh = r.Header.Get("X-Refresh-Token")
	}
	if refresh != "" {
		if claims, err := h.jwt.Parse(refresh); err == nil && claims.ID != "" {
			_ = h.store.RevokeRefreshToken(r.Context(), claims.ID)
		}
	}
	clearRefreshCookie(w, h.cookieHost)
	jsonOut(w, map[string]bool{"success": true}, 200)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	user, err := h.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		jsonOut(w, map[string]string{"error": "user not found"}, 404)
		return
	}
	jsonOut(w, user, 200)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	var d struct {
		Name string `json:"name"`
	}
	decode(r, &d)
	if strings.TrimSpace(d.Name) == "" {
		jsonOut(w, map[string]string{"error": "name is required"}, 400)
		return
	}
	user, err := h.store.UpdateName(r.Context(), claims.UserID, d.Name)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to update profile"}, 500)
		return
	}
	jsonOut(w, user, 200)
}

func (h *Handler) TelegramLoginCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if h.botToken == "" {
		jsonOut(w, map[string]string{"error": "telegram bot token is not configured"}, 400)
		return
	}
	code := telegramCode()
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	if err := h.store.SaveTelegramLoginCode(r.Context(), code, expiresAt); err != nil {
		jsonOut(w, map[string]string{"error": "failed to create telegram login code"}, 500)
		return
	}
	botUsername := ""
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	if bot, err := telegram.NewWithProxy(h.botToken, h.botProxy).GetMe(ctx); err == nil {
		botUsername = bot.Username
	}
	cancel()
	jsonOut(w, map[string]any{"code": code, "expires_at": expiresAt.Format(time.RFC3339), "bot_username": botUsername, "start_command": "/start", "deep_link": telegramDeepLink(botUsername, code)}, 200)
}

func (h *Handler) TelegramConfirmLoginCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var d struct {
		Code string `json:"code"`
	}
	decode(r, &d)
	if h.botToken == "" {
		jsonOut(w, map[string]string{"error": "telegram bot token is not configured"}, 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	chat, err := telegram.NewWithProxy(h.botToken, h.botProxy).FindStartCode(ctx, d.Code)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 404)
		return
	}
	if err := h.store.ConsumeTelegramLoginCode(r.Context(), d.Code); err != nil {
		jsonOut(w, map[string]string{"error": "telegram login code is expired or invalid"}, 400)
		return
	}
	user, err := h.store.UpsertTelegramUser(r.Context(), chat.ID, chat.ID, telegramDisplayName(chat))
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to save telegram user"}, 500)
		return
	}
	access, refresh, err := h.issueTokens(r.Context(), user)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to issue tokens"}, 500)
		return
	}
	setRefreshCookie(w, refresh, h.cookieHost)
	jsonOut(w, map[string]any{"user": user, "access_token": access}, 200)
}

func (h *Handler) TelegramLinkCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	code := telegramCode()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := h.store.SaveTelegramLinkCode(r.Context(), claims.UserID, code, expiresAt); err != nil {
		jsonOut(w, map[string]string{"error": "failed to create telegram link code"}, 500)
		return
	}
	botUsername := ""
	if h.botToken != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		if bot, err := telegram.NewWithProxy(h.botToken, h.botProxy).GetMe(ctx); err == nil {
			botUsername = bot.Username
		}
		cancel()
	}
	jsonOut(w, map[string]any{"code": code, "expires_at": expiresAt.Format(time.RFC3339), "bot_username": botUsername, "start_command": "/start", "deep_link": telegramDeepLink(botUsername, code)}, 200)
}

func (h *Handler) TelegramConfirmCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	var d struct {
		Code string `json:"code"`
	}
	decode(r, &d)
	if h.botToken == "" {
		jsonOut(w, map[string]string{"error": "telegram bot token is not configured"}, 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	chat, err := telegram.NewWithProxy(h.botToken, h.botProxy).FindStartCode(ctx, d.Code)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	username := telegramDisplayName(chat)
	user, err := h.store.LinkTelegramByCode(r.Context(), d.Code, chat.ID, chat.ID, username)
	if err != nil {
		jsonOut(w, map[string]string{"error": err.Error()}, 400)
		return
	}
	if user.ID != claims.UserID {
		jsonOut(w, map[string]string{"error": "invalid linked user"}, 403)
		return
	}
	_ = telegram.NewWithProxy(h.botToken, h.botProxy).SendMessage(context.Background(), strconv.FormatInt(chat.ID, 10), "✅ Funpay Parser: Telegram привязан к вашему аккаунту.")
	jsonOut(w, user, 200)
}

func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	user, err := h.store.CompleteOnboarding(r.Context(), claims.UserID)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to complete onboarding"}, 500)
		return
	}
	jsonOut(w, user, 200)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	var d struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	decode(r, &d)
	if len(d.NewPassword) < 6 {
		jsonOut(w, map[string]string{"error": "new password must be at least 6 chars"}, 400)
		return
	}
	user, hash, err := h.store.GetUserByEmail(r.Context(), claims.Email)
	if err != nil {
		jsonOut(w, map[string]string{"error": "user not found"}, 404)
		return
	}
	if user.ID != claims.UserID {
		jsonOut(w, map[string]string{"error": "invalid user"}, 403)
		return
	}
	if hash != "" && !authstore.CheckPassword(hash, d.CurrentPassword) {
		jsonOut(w, map[string]string{"error": "current password is invalid"}, 401)
		return
	}
	if err := h.store.UpdatePassword(r.Context(), claims.UserID, d.NewPassword); err != nil {
		jsonOut(w, map[string]string{"error": "failed to update password"}, 500)
		return
	}
	_ = h.store.RevokeAllUserRefreshTokens(r.Context(), claims.UserID)
	clearRefreshCookie(w, h.cookieHost)
	jsonOut(w, map[string]bool{"success": true}, 200)
}

func (h *Handler) TelegramLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var d struct {
		InitData string `json:"init_data"`
	}
	decode(r, &d)
	if d.InitData == "" {
		jsonOut(w, map[string]string{"error": "init_data is required"}, 400)
		return
	}
	userInfo, chatID, ok, err := telegramverify.VerifyInitData(d.InitData, h.botToken)
	if !ok || err != nil {
		jsonOut(w, map[string]string{"error": "invalid telegram initData"}, 401)
		return
	}
	user, err := h.store.UpsertTelegramUser(r.Context(), userInfo.ID, chatID, userInfo.Username)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to save telegram user"}, 500)
		return
	}
	access, refresh, err := h.issueTokens(r.Context(), user)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to issue tokens"}, 500)
		return
	}
	setRefreshCookie(w, refresh, h.cookieHost)
	jsonOut(w, map[string]any{"user": user, "access_token": access}, 200)
}

func (h *Handler) TelegramLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	claims := r.Context().Value("claims").(*jwt.Claims)
	var d struct {
		InitData string `json:"init_data"`
	}
	decode(r, &d)
	if d.InitData == "" {
		jsonOut(w, map[string]string{"error": "init_data is required"}, 400)
		return
	}
	userInfo, chatID, ok, err := telegramverify.VerifyInitData(d.InitData, h.botToken)
	if !ok || err != nil {
		jsonOut(w, map[string]string{"error": "invalid telegram initData"}, 401)
		return
	}
	user, err := h.store.LinkTelegram(r.Context(), claims.UserID, userInfo.ID, chatID, userInfo.Username)
	if err != nil {
		jsonOut(w, map[string]string{"error": "failed to link telegram"}, 500)
		return
	}
	jsonOut(w, user, 200)
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if auth := r.Header.Get("Authorization"); auth != "" {
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
		if token == "" {
			if c, err := r.Cookie("access_token"); err == nil {
				token = c.Value
			}
		}
		if token == "" {
			jsonOut(w, map[string]string{"error": "authorization required"}, 401)
			return
		}
		claims, err := h.jwt.ValidateAccess(token)
		if err != nil {
			jsonOut(w, map[string]string{"error": "invalid or expired token"}, 401)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "claims", claims)))
	})
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/config", h.ConfigInfo)
	mux.HandleFunc("/api/auth/register", h.Register)
	mux.HandleFunc("/api/auth/login", h.Login)
	mux.HandleFunc("/api/auth/refresh", h.Refresh)
	mux.HandleFunc("/api/auth/logout", h.Logout)
	mux.HandleFunc("/api/auth/telegram/login", h.TelegramLogin)
	mux.HandleFunc("/api/auth/telegram/login-code", h.TelegramLoginCode)
	mux.HandleFunc("/api/auth/telegram/confirm-login-code", h.TelegramConfirmLoginCode)
	mux.Handle("/api/auth/telegram/link", h.AuthMiddleware(http.HandlerFunc(h.TelegramLink)))
	mux.Handle("/api/auth/profile", h.AuthMiddleware(http.HandlerFunc(h.UpdateProfile)))
	mux.Handle("/api/auth/telegram/link-code", h.AuthMiddleware(http.HandlerFunc(h.TelegramLinkCode)))
	mux.Handle("/api/auth/telegram/confirm-code", h.AuthMiddleware(http.HandlerFunc(h.TelegramConfirmCode)))
	mux.Handle("/api/auth/password", h.AuthMiddleware(http.HandlerFunc(h.ChangePassword)))
	mux.Handle("/api/auth/onboarding", h.AuthMiddleware(http.HandlerFunc(h.CompleteOnboarding)))
	mux.Handle("/api/auth/me", h.AuthMiddleware(http.HandlerFunc(h.Me)))
	return mux
}
