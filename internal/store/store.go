package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

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
		`CREATE TABLE IF NOT EXISTS profiles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 1, name TEXT NOT NULL, query TEXT NOT NULL DEFAULT 'chatgpt plus', category_id INTEGER NOT NULL DEFAULT 1355, candidates INTEGER NOT NULL DEFAULT 40, max_pages INTEGER NULL, deep INTEGER NOT NULL DEFAULT 0, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS saved_results (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 1, profile_id INTEGER NOT NULL, run_at TEXT DEFAULT CURRENT_TIMESTAMP, cheapest_json TEXT NULL, summary_json TEXT NULL, all_results_json TEXT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS schedules (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL DEFAULT 1, profile_id INTEGER NOT NULL, interval_minutes INTEGER NOT NULL DEFAULT 60, enabled INTEGER NOT NULL DEFAULT 1, next_run_at TEXT NULL, last_run_at TEXT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, st := range stmts {
		if _, err := s.DB.ExecContext(ctx, st); err != nil {
			return err
		}
	}
	migrations := []string{
		`ALTER TABLE profiles ADD COLUMN user_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE saved_results ADD COLUMN user_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE schedules ADD COLUMN user_id INTEGER NOT NULL DEFAULT 1`,
		`CREATE INDEX IF NOT EXISTS idx_profiles_user ON profiles(user_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_user_run ON saved_results(user_id, run_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_user_profile ON saved_results(user_id, profile_id)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_user ON schedules(user_id, updated_at DESC)`,
	}
	for _, st := range migrations {
		_, _ = s.DB.ExecContext(ctx, st)
	}
	_ = s.PruneAllSaved(ctx, 10)
	return nil
}

type Profile struct {
	ID         int    `json:"id"`
	UserID     int64  `json:"user_id,omitempty"`
	Name       string `json:"name"`
	Query      string `json:"query"`
	CategoryID int    `json:"category_id"`
	Candidates int    `json:"candidates"`
	MaxPages   *int   `json:"max_pages"`
	Deep       bool   `json:"deep"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
type Schedule struct {
	ID              int     `json:"id"`
	UserID          int64   `json:"user_id,omitempty"`
	ProfileID       int     `json:"profile_id"`
	IntervalMinutes int     `json:"interval_minutes"`
	Enabled         bool    `json:"enabled"`
	NextRunAt       *string `json:"next_run_at"`
	LastRunAt       *string `json:"last_run_at"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	ProfileName     string  `json:"profile_name,omitempty"`
}
type SavedResult struct {
	ID         int    `json:"id"`
	UserID     int64  `json:"user_id,omitempty"`
	ProfileID  int    `json:"profile_id"`
	RunAt      string `json:"run_at"`
	Cheapest   any    `json:"cheapest"`
	Summary    any    `json:"summary"`
	AllResults any    `json:"all_results,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type UserStats struct {
	Profiles       int     `json:"profiles"`
	SavedResults   int     `json:"saved_results"`
	Schedules      int     `json:"schedules"`
	TotalLLM       int     `json:"total_llm"`
	TotalPlus      int     `json:"total_plus"`
	BestPrice      float64 `json:"best_price,omitempty"`
	BestCurrency   string  `json:"best_currency,omitempty"`
	LastRunAt      string  `json:"last_run_at,omitempty"`
	TelegramLinked bool    `json:"telegram_linked"`
}

func (s *Store) ListProfiles(ctx context.Context, userID int64) ([]Profile, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,user_id,name,query,category_id,candidates,max_pages,deep,created_at,updated_at FROM profiles WHERE user_id=? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var p Profile
		var max sql.NullInt64
		var deep int
		_ = rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Query, &p.CategoryID, &p.Candidates, &max, &deep, &p.CreatedAt, &p.UpdatedAt)
		if max.Valid {
			v := int(max.Int64)
			p.MaxPages = &v
		}
		p.Deep = deep != 0
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) GetProfile(ctx context.Context, userID int64, id int) (*Profile, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id,user_id,name,query,category_id,candidates,max_pages,deep,created_at,updated_at FROM profiles WHERE user_id=? AND id=?`, userID, id)
	var p Profile
	var max sql.NullInt64
	var deep int
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Query, &p.CategoryID, &p.Candidates, &max, &deep, &p.CreatedAt, &p.UpdatedAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if max.Valid {
		v := int(max.Int64)
		p.MaxPages = &v
	}
	p.Deep = deep != 0
	return &p, nil
}

func (s *Store) EnsureDefaultProfile(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return nil
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=?`, userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	maxPages := 5
	_, err := s.CreateProfile(ctx, userID, Profile{
		Name:       "ChatGPT Plus personal",
		Query:      "chatgpt plus 30 дней",
		CategoryID: 1355,
		Candidates: 10,
		MaxPages:   &maxPages,
		Deep:       true,
	})
	return err
}

func (s *Store) CreateProfile(ctx context.Context, userID int64, p Profile) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO profiles(user_id,name,query,category_id,candidates,max_pages,deep,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, userID, p.Name, p.Query, p.CategoryID, p.Candidates, p.MaxPages, boolInt(p.Deep), now, now)
	if err != nil {
		return p, err
	}
	id, _ := r.LastInsertId()
	p.ID = int(id)
	p.UserID = userID
	p.CreatedAt = now
	p.UpdatedAt = now
	return p, nil
}
func (s *Store) UpdateProfile(ctx context.Context, userID int64, id int, p Profile) (*Profile, error) {
	_, err := s.DB.ExecContext(ctx, `UPDATE profiles SET name=COALESCE(NULLIF(?,''),name), query=COALESCE(NULLIF(?,''),query), category_id=CASE WHEN ?>0 THEN ? ELSE category_id END, candidates=CASE WHEN ?>0 THEN ? ELSE candidates END, max_pages=?, deep=?, updated_at=? WHERE user_id=? AND id=?`, p.Name, p.Query, p.CategoryID, p.CategoryID, p.Candidates, p.Candidates, p.MaxPages, boolInt(p.Deep), time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, userID, id)
}
func (s *Store) DeleteProfile(ctx context.Context, userID int64, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM profiles WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

func (s *Store) SaveResult(ctx context.Context, userID int64, profileID int, cheapest, summary, all any) (SavedResult, error) {
	cj, _ := json.Marshal(cheapest)
	sj, _ := json.Marshal(summary)
	aj, _ := json.Marshal(all)
	now := time.Now().UTC().Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO saved_results(user_id,profile_id,run_at,cheapest_json,summary_json,all_results_json,created_at) VALUES(?,?,?,?,?,?,?)`, userID, profileID, now, string(cj), string(sj), string(aj), now)
	if err != nil {
		return SavedResult{}, err
	}
	id, _ := r.LastInsertId()
	_ = s.PruneSaved(ctx, userID, 10)
	return SavedResult{ID: int(id), UserID: userID, ProfileID: profileID, RunAt: now, Cheapest: cheapest, Summary: summary, CreatedAt: now}, nil
}
func (s *Store) PruneSaved(ctx context.Context, userID int64, keep int) error {
	if keep < 1 {
		keep = 10
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM saved_results WHERE user_id=? AND id NOT IN (SELECT id FROM saved_results WHERE user_id=? ORDER BY run_at DESC, id DESC LIMIT ?)`, userID, userID, keep)
	return err
}
func (s *Store) PruneAllSaved(ctx context.Context, keep int) error {
	if keep < 1 {
		keep = 10
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT user_id FROM saved_results`)
	if err != nil {
		return err
	}
	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}
	_ = rows.Close()
	for _, userID := range userIDs {
		if err := s.PruneSaved(ctx, userID, keep); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListSaved(ctx context.Context, userID int64, profileID int) ([]SavedResult, error) {
	q := `SELECT id,user_id,profile_id,run_at,cheapest_json,summary_json,created_at FROM saved_results WHERE user_id=?`
	args := []any{userID}
	if profileID > 0 {
		q += ` AND profile_id=?`
		args = append(args, profileID)
	}
	q += ` ORDER BY run_at DESC, id DESC LIMIT 10`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedResult{}
	for rows.Next() {
		var r SavedResult
		var c, sm string
		_ = rows.Scan(&r.ID, &r.UserID, &r.ProfileID, &r.RunAt, &c, &sm, &r.CreatedAt)
		_ = json.Unmarshal([]byte(c), &r.Cheapest)
		_ = json.Unmarshal([]byte(sm), &r.Summary)
		out = append(out, r)
	}
	return out, nil
}
func (s *Store) GetSaved(ctx context.Context, userID int64, id int) (*SavedResult, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id,user_id,profile_id,run_at,cheapest_json,summary_json,all_results_json,created_at FROM saved_results WHERE user_id=? AND id=?`, userID, id)
	var r SavedResult
	var c, sm, all string
	if err := row.Scan(&r.ID, &r.UserID, &r.ProfileID, &r.RunAt, &c, &sm, &all, &r.CreatedAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(c), &r.Cheapest)
	_ = json.Unmarshal([]byte(sm), &r.Summary)
	_ = json.Unmarshal([]byte(all), &r.AllResults)
	return &r, nil
}
func (s *Store) DeleteSaved(ctx context.Context, userID int64, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM saved_results WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

func (s *Store) ListSchedules(ctx context.Context, userID int64) ([]Schedule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id,s.user_id,s.profile_id,s.interval_minutes,s.enabled,s.next_run_at,s.last_run_at,s.created_at,s.updated_at,COALESCE(p.name,'') FROM schedules s LEFT JOIN profiles p ON p.id=s.profile_id AND p.user_id=s.user_id WHERE s.user_id=? ORDER BY s.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		var sc Schedule
		var en int
		var next, last sql.NullString
		_ = rows.Scan(&sc.ID, &sc.UserID, &sc.ProfileID, &sc.IntervalMinutes, &en, &next, &last, &sc.CreatedAt, &sc.UpdatedAt, &sc.ProfileName)
		sc.Enabled = en != 0
		if next.Valid {
			sc.NextRunAt = &next.String
		}
		if last.Valid {
			sc.LastRunAt = &last.String
		}
		out = append(out, sc)
	}
	return out, nil
}
func (s *Store) GetSchedule(ctx context.Context, userID int64, id int) (*Schedule, error) {
	rows, err := s.ListSchedules(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, sc := range rows {
		if sc.ID == id {
			return &sc, nil
		}
	}
	return nil, nil
}
func (s *Store) CreateSchedule(ctx context.Context, userID int64, sc Schedule) (Schedule, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	next := time.Now().UTC().Add(time.Duration(sc.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO schedules(user_id,profile_id,interval_minutes,enabled,next_run_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, userID, sc.ProfileID, sc.IntervalMinutes, boolInt(sc.Enabled), next, now, now)
	if err != nil {
		return sc, err
	}
	id, _ := r.LastInsertId()
	sc.ID = int(id)
	sc.UserID = userID
	sc.CreatedAt = now
	sc.UpdatedAt = now
	sc.NextRunAt = &next
	return sc, nil
}
func (s *Store) UpdateSchedule(ctx context.Context, userID int64, id int, sc Schedule) (*Schedule, error) {
	next := time.Now().UTC().Add(time.Duration(sc.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `UPDATE schedules SET interval_minutes=CASE WHEN ?>0 THEN ? ELSE interval_minutes END, enabled=?, next_run_at=CASE WHEN ?>0 THEN ? ELSE next_run_at END, updated_at=? WHERE user_id=? AND id=?`, sc.IntervalMinutes, sc.IntervalMinutes, boolInt(sc.Enabled), sc.IntervalMinutes, next, time.Now().UTC().Format(time.RFC3339), userID, id)
	if err != nil {
		return nil, err
	}
	return s.GetSchedule(ctx, userID, id)
}
func (s *Store) DeleteSchedule(ctx context.Context, userID int64, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM schedules WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *Store) TouchScheduleRun(ctx context.Context, userID int64, id int, next time.Time) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE schedules SET last_run_at=?, next_run_at=? WHERE user_id=? AND id=?`, time.Now().UTC().Format(time.RFC3339), next.UTC().Format(time.RFC3339), userID, id)
}

func (s *Store) Stats(ctx context.Context, userID int64, telegramLinked bool) (UserStats, error) {
	var st UserStats
	st.TelegramLinked = telegramLinked
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=?`, userID).Scan(&st.Profiles)
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM saved_results WHERE user_id=?`, userID).Scan(&st.SavedResults)
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schedules WHERE user_id=?`, userID).Scan(&st.Schedules)
	rows, err := s.DB.QueryContext(ctx, `SELECT run_at, cheapest_json, summary_json FROM saved_results WHERE user_id=? ORDER BY run_at DESC, id DESC LIMIT 10`, userID)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	bestSet := false
	for rows.Next() {
		var runAt, cheapestJSON, summaryJSON string
		_ = rows.Scan(&runAt, &cheapestJSON, &summaryJSON)
		if st.LastRunAt == "" {
			st.LastRunAt = runAt
		}
		var summary map[string]any
		if json.Unmarshal([]byte(summaryJSON), &summary) == nil {
			st.TotalLLM += intFromAny(summary["classified"])
			st.TotalPlus += intFromAny(summary["total_plus"])
		}
		var cheapest map[string]any
		if json.Unmarshal([]byte(cheapestJSON), &cheapest) == nil {
			price := floatFromAny(cheapest["price"])
			if price > 0 && (!bestSet || price < st.BestPrice) {
				bestSet = true
				st.BestPrice = price
				if c, ok := cheapest["currency"].(string); ok {
					st.BestCurrency = c
				}
			}
		}
	}
	return st, nil
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}
func floatFromAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) ListAllSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id,s.user_id,s.profile_id,s.interval_minutes,s.enabled,s.next_run_at,s.last_run_at,s.created_at,s.updated_at,COALESCE(p.name,'') FROM schedules s LEFT JOIN profiles p ON p.id=s.profile_id AND p.user_id=s.user_id ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		var sc Schedule
		var en int
		var next, last sql.NullString
		_ = rows.Scan(&sc.ID, &sc.UserID, &sc.ProfileID, &sc.IntervalMinutes, &en, &next, &last, &sc.CreatedAt, &sc.UpdatedAt, &sc.ProfileName)
		sc.Enabled = en != 0
		if next.Valid {
			sc.NextRunAt = &next.String
		}
		if last.Valid {
			sc.LastRunAt = &last.String
		}
		out = append(out, sc)
	}
	return out, nil
}
