package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"funpay-parser/internal/auth/jwt"
	authstore "funpay-parser/internal/auth/store"
	"funpay-parser/internal/auth/telegramverify"
)

type Handler struct {
	store      *authstore.Store
	jwt        *jwt.Manager
	botToken   string
	cookieHost string
}

type Config struct {
	Store      *authstore.Store
	JWTManager *jwt.Manager
	BotToken   string
	CookieHost string
}

func New(cfg Config) *Handler {
	return &Handler{store: cfg.Store, jwt: cfg.JWTManager, botToken: cfg.BotToken, cookieHost: cfg.CookieHost}
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
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decode(r, &d)
	if d.Email == "" || d.Password == "" || len(d.Password) < 6 {
		jsonOut(w, map[string]string{"error": "email and password (min 6 chars) are required"}, 400)
		return
	}
	user, err := h.store.CreateUser(r.Context(), d.Email, d.Password)
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
	mux.Handle("/api/auth/telegram/link", h.AuthMiddleware(http.HandlerFunc(h.TelegramLink)))
	mux.Handle("/api/auth/me", h.AuthMiddleware(http.HandlerFunc(h.Me)))
	return mux
}
