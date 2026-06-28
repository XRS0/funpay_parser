package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

type User struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	TelegramUserID   int64  `json:"telegram_user_id,omitempty"`
	TelegramChatID   int64  `json:"telegram_chat_id,omitempty"`
	TelegramUsername string `json:"telegram_username,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db}
	return s, s.Init(context.Background())
}

func (s *Store) Init(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			telegram_user_id INTEGER NULL UNIQUE,
			telegram_chat_id INTEGER NULL,
			telegram_username TEXT NULL,
			telegram_link_code TEXT NULL UNIQUE,
			telegram_link_expires_at TEXT NULL,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_user ON refresh_tokens(user_id)`,
		`CREATE TABLE IF NOT EXISTS telegram_login_codes (code TEXT PRIMARY KEY, expires_at TEXT NOT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, st := range stmts {
		if _, err := s.DB.ExecContext(ctx, st); err != nil {
			return err
		}
	}
	migrations := []string{
		`ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN telegram_link_code TEXT NULL`,
		`ALTER TABLE users ADD COLUMN telegram_link_expires_at TEXT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_telegram_link_code ON users(telegram_link_code)`,
	}
	for _, st := range migrations {
		_, _ = s.DB.ExecContext(ctx, st)
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, name, email, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	name = strings.TrimSpace(name)
	if name == "" {
		name = email
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(name, email, password_hash, role, created_at, updated_at) VALUES(?,?,?,?,?,?)`, name, email, string(hash), "user", now, now)
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Name: name, Email: email, Role: "user", CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, string, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at, password_hash FROM users WHERE email=?`, email)
	var u User
	var hash string
	var tuid, tcid sql.NullInt64
	var tname sql.NullString
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &tuid, &tcid, &tname, &u.CreatedAt, &u.UpdatedAt, &hash)
	if err == sql.ErrNoRows {
		return User{}, "", sql.ErrNoRows
	}
	if err != nil {
		return User{}, "", err
	}
	if tuid.Valid {
		u.TelegramUserID = tuid.Int64
	}
	if tcid.Valid {
		u.TelegramChatID = tcid.Int64
	}
	if tname.Valid {
		u.TelegramUsername = tname.String
	}
	return u, hash, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, name, email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at FROM users WHERE id=?`, id)
	var u User
	var tuid, tcid sql.NullInt64
	var tname sql.NullString
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &tuid, &tcid, &tname, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return User{}, sql.ErrNoRows
	}
	if err != nil {
		return User{}, err
	}
	if tuid.Valid {
		u.TelegramUserID = tuid.Int64
	}
	if tcid.Valid {
		u.TelegramChatID = tcid.Int64
	}
	if tname.Valid {
		u.TelegramUsername = tname.String
	}
	return u, nil
}

func (s *Store) LinkTelegram(ctx context.Context, userID int64, telegramUserID int64, chatID int64, username string) (User, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET telegram_user_id=?, telegram_chat_id=?, telegram_username=?, updated_at=? WHERE id=?`, telegramUserID, chatID, username, now, userID)
	if err != nil {
		return User{}, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) UpsertTelegramUser(ctx context.Context, telegramUserID, chatID int64, username string) (User, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(name, email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(telegram_user_id) DO UPDATE SET telegram_chat_id=excluded.telegram_chat_id, telegram_username=excluded.telegram_username, updated_at=excluded.updated_at`, telegramName(username, telegramUserID), fmt.Sprintf("tg_%d", telegramUserID), "user", telegramUserID, chatID, username, now, now)
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		row := s.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE telegram_user_id=?`, telegramUserID)
		_ = row.Scan(&id)
	}
	return s.GetUserByID(ctx, id)
}

func (s *Store) UpdateName(ctx context.Context, userID int64, name string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return s.GetUserByID(ctx, userID)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET name=?, updated_at=? WHERE id=?`, name, now, userID)
	if err != nil {
		return User{}, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) SaveTelegramLoginCode(ctx context.Context, code string, expiresAt time.Time) error {
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM telegram_login_codes WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	_, err := s.DB.ExecContext(ctx, `INSERT OR REPLACE INTO telegram_login_codes(code, expires_at, created_at) VALUES(?,?,?)`, strings.TrimSpace(code), expiresAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) ConsumeTelegramLoginCode(ctx context.Context, code string) error {
	code = strings.TrimSpace(code)
	row := s.DB.QueryRowContext(ctx, `SELECT expires_at FROM telegram_login_codes WHERE code=?`, code)
	var exp string
	if err := row.Scan(&exp); err != nil {
		return err
	}
	expiresAt, _ := time.Parse(time.RFC3339, exp)
	if !expiresAt.IsZero() && time.Now().UTC().After(expiresAt) {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM telegram_login_codes WHERE code=?`, code)
		return fmt.Errorf("telegram login code expired")
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM telegram_login_codes WHERE code=?`, code)
	return err
}

func (s *Store) SaveTelegramLinkCode(ctx context.Context, userID int64, code string, expiresAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET telegram_link_code=?, telegram_link_expires_at=?, updated_at=? WHERE id=?`, code, expiresAt.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), userID)
	return err
}

func (s *Store) LinkTelegramByCode(ctx context.Context, code string, telegramUserID int64, chatID int64, username string) (User, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return User{}, sql.ErrNoRows
	}
	row := s.DB.QueryRowContext(ctx, `SELECT id, telegram_link_expires_at FROM users WHERE telegram_link_code=?`, code)
	var userID int64
	var exp string
	if err := row.Scan(&userID, &exp); err != nil {
		return User{}, err
	}
	expiresAt, _ := time.Parse(time.RFC3339, exp)
	if !expiresAt.IsZero() && time.Now().UTC().After(expiresAt) {
		return User{}, fmt.Errorf("telegram link code expired")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET telegram_user_id=?, telegram_chat_id=?, telegram_username=?, telegram_link_code=NULL, telegram_link_expires_at=NULL, updated_at=? WHERE id=?`, telegramUserID, chatID, username, now, userID)
	if err != nil {
		return User{}, err
	}
	return s.GetUserByID(ctx, userID)
}

func telegramName(username string, id int64) string {
	username = strings.TrimSpace(username)
	if username != "" {
		return "@" + username
	}
	return fmt.Sprintf("Telegram %d", id)
}

func (s *Store) SaveRefreshToken(ctx context.Context, tokenID string, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO refresh_tokens(id, user_id, token_hash, expires_at) VALUES(?,?,?,?)`, tokenID, userID, tokenHash, expiresAt.UTC().Format(time.RFC3339))
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, tokenID string) (userID int64, tokenHash string, revoked bool, expiresAt time.Time, err error) {
	row := s.DB.QueryRowContext(ctx, `SELECT user_id, token_hash, revoked, expires_at FROM refresh_tokens WHERE id=?`, tokenID)
	var exp string
	var rev int
	err = row.Scan(&userID, &tokenHash, &rev, &exp)
	if err == sql.ErrNoRows {
		return 0, "", false, time.Time{}, sql.ErrNoRows
	}
	if err != nil {
		return 0, "", false, time.Time{}, err
	}
	expiresAt, _ = time.Parse(time.RFC3339, exp)
	return userID, tokenHash, rev != 0, expiresAt, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE refresh_tokens SET revoked=1 WHERE id=?`, tokenID)
	return err
}

func (s *Store) RevokeAllUserRefreshTokens(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE refresh_tokens SET revoked=1 WHERE user_id=?`, userID)
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, userID int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.DB.ExecContext(ctx, `UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, string(hash), now, userID)
	return err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
