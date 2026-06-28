package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

type User struct {
	ID               int64  `json:"id"`
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
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			telegram_user_id INTEGER NULL UNIQUE,
			telegram_chat_id INTEGER NULL,
			telegram_username TEXT NULL,
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
	}
	for _, st := range stmts {
		if _, err := s.DB.ExecContext(ctx, st); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(email, password_hash, role, created_at, updated_at) VALUES(?,?,?,?,?)`, email, string(hash), "user", now, now)
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Email: email, Role: "user", CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, string, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at, password_hash FROM users WHERE email=?`, email)
	var u User
	var hash string
	var tuid, tcid sql.NullInt64
	var tname sql.NullString
	err := row.Scan(&u.ID, &u.Email, &u.Role, &tuid, &tcid, &tname, &u.CreatedAt, &u.UpdatedAt, &hash)
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
	row := s.DB.QueryRowContext(ctx, `SELECT id, email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at FROM users WHERE id=?`, id)
	var u User
	var tuid, tcid sql.NullInt64
	var tname sql.NullString
	err := row.Scan(&u.ID, &u.Email, &u.Role, &tuid, &tcid, &tname, &u.CreatedAt, &u.UpdatedAt)
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
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users(email, role, telegram_user_id, telegram_chat_id, telegram_username, created_at, updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(telegram_user_id) DO UPDATE SET telegram_chat_id=excluded.telegram_chat_id, telegram_username=excluded.telegram_username, updated_at=excluded.updated_at`, fmt.Sprintf("tg_%d", telegramUserID), "user", telegramUserID, chatID, username, now, now)
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
