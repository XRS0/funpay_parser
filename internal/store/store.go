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
		`CREATE TABLE IF NOT EXISTS profiles (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, query TEXT NOT NULL DEFAULT 'chatgpt plus', category_id INTEGER NOT NULL DEFAULT 1355, candidates INTEGER NOT NULL DEFAULT 40, max_pages INTEGER NULL, deep INTEGER NOT NULL DEFAULT 0, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS saved_results (id INTEGER PRIMARY KEY AUTOINCREMENT, profile_id INTEGER NOT NULL, run_at TEXT DEFAULT CURRENT_TIMESTAMP, cheapest_json TEXT NULL, summary_json TEXT NULL, all_results_json TEXT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS schedules (id INTEGER PRIMARY KEY AUTOINCREMENT, profile_id INTEGER NOT NULL, interval_minutes INTEGER NOT NULL DEFAULT 60, enabled INTEGER NOT NULL DEFAULT 1, next_run_at TEXT NULL, last_run_at TEXT NULL, created_at TEXT DEFAULT CURRENT_TIMESTAMP, updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`}
	for _, st := range stmts {
		if _, err := s.DB.ExecContext(ctx, st); err != nil {
			return err
		}
	}
	return nil
}

type Profile struct {
	ID         int    `json:"id"`
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
	ProfileID  int    `json:"profile_id"`
	RunAt      string `json:"run_at"`
	Cheapest   any    `json:"cheapest"`
	Summary    any    `json:"summary"`
	AllResults any    `json:"all_results,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (s *Store) ListProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,name,query,category_id,candidates,max_pages,deep,created_at,updated_at FROM profiles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		var p Profile
		var max sql.NullInt64
		var deep int
		rows.Scan(&p.ID, &p.Name, &p.Query, &p.CategoryID, &p.Candidates, &max, &deep, &p.CreatedAt, &p.UpdatedAt)
		if max.Valid {
			v := int(max.Int64)
			p.MaxPages = &v
		}
		p.Deep = deep != 0
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) GetProfile(ctx context.Context, id int) (*Profile, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,name,query,category_id,candidates,max_pages,deep,created_at,updated_at FROM profiles WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var p Profile
	var max sql.NullInt64
	var deep int
	rows.Scan(&p.ID, &p.Name, &p.Query, &p.CategoryID, &p.Candidates, &max, &deep, &p.CreatedAt, &p.UpdatedAt)
	if max.Valid {
		v := int(max.Int64)
		p.MaxPages = &v
	}
	p.Deep = deep != 0
	return &p, nil
}
func (s *Store) CreateProfile(ctx context.Context, p Profile) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO profiles(name,query,category_id,candidates,max_pages,deep,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, p.Name, p.Query, p.CategoryID, p.Candidates, p.MaxPages, boolInt(p.Deep), now, now)
	if err != nil {
		return p, err
	}
	id, _ := r.LastInsertId()
	p.ID = int(id)
	p.CreatedAt = now
	p.UpdatedAt = now
	return p, nil
}
func (s *Store) UpdateProfile(ctx context.Context, id int, p Profile) (*Profile, error) {
	_, err := s.DB.ExecContext(ctx, `UPDATE profiles SET name=COALESCE(NULLIF(?,''),name), query=COALESCE(NULLIF(?,''),query), category_id=CASE WHEN ?>0 THEN ? ELSE category_id END, candidates=CASE WHEN ?>0 THEN ? ELSE candidates END, max_pages=?, deep=?, updated_at=? WHERE id=?`, p.Name, p.Query, p.CategoryID, p.CategoryID, p.Candidates, p.Candidates, p.MaxPages, boolInt(p.Deep), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}
func (s *Store) DeleteProfile(ctx context.Context, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM profiles WHERE id=?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

func (s *Store) SaveResult(ctx context.Context, profileID int, cheapest, summary, all any) (SavedResult, error) {
	cj, _ := json.Marshal(cheapest)
	sj, _ := json.Marshal(summary)
	aj, _ := json.Marshal(all)
	now := time.Now().UTC().Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO saved_results(profile_id,run_at,cheapest_json,summary_json,all_results_json,created_at) VALUES(?,?,?,?,?,?)`, profileID, now, string(cj), string(sj), string(aj), now)
	if err != nil {
		return SavedResult{}, err
	}
	id, _ := r.LastInsertId()
	return SavedResult{ID: int(id), ProfileID: profileID, RunAt: now, Cheapest: cheapest, Summary: summary, CreatedAt: now}, nil
}
func (s *Store) ListSaved(ctx context.Context, profileID int) ([]SavedResult, error) {
	q := `SELECT id,profile_id,run_at,cheapest_json,summary_json,created_at FROM saved_results`
	args := []any{}
	if profileID > 0 {
		q += ` WHERE profile_id=?`
		args = append(args, profileID)
	}
	q += ` ORDER BY run_at DESC`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedResult{}
	for rows.Next() {
		var r SavedResult
		var c, sm string
		rows.Scan(&r.ID, &r.ProfileID, &r.RunAt, &c, &sm, &r.CreatedAt)
		_ = json.Unmarshal([]byte(c), &r.Cheapest)
		_ = json.Unmarshal([]byte(sm), &r.Summary)
		out = append(out, r)
	}
	return out, nil
}
func (s *Store) GetSaved(ctx context.Context, id int) (*SavedResult, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id,profile_id,run_at,cheapest_json,summary_json,all_results_json,created_at FROM saved_results WHERE id=?`, id)
	var r SavedResult
	var c, sm, all string
	if err := row.Scan(&r.ID, &r.ProfileID, &r.RunAt, &c, &sm, &all, &r.CreatedAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(c), &r.Cheapest)
	_ = json.Unmarshal([]byte(sm), &r.Summary)
	_ = json.Unmarshal([]byte(all), &r.AllResults)
	return &r, nil
}
func (s *Store) DeleteSaved(ctx context.Context, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM saved_results WHERE id=?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

func (s *Store) ListSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id,s.profile_id,s.interval_minutes,s.enabled,s.next_run_at,s.last_run_at,s.created_at,s.updated_at,COALESCE(p.name,'') FROM schedules s LEFT JOIN profiles p ON p.id=s.profile_id ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		var sc Schedule
		var en int
		var next, last sql.NullString
		rows.Scan(&sc.ID, &sc.ProfileID, &sc.IntervalMinutes, &en, &next, &last, &sc.CreatedAt, &sc.UpdatedAt, &sc.ProfileName)
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
func (s *Store) GetSchedule(ctx context.Context, id int) (*Schedule, error) {
	rows, err := s.ListSchedules(ctx)
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
func (s *Store) CreateSchedule(ctx context.Context, sc Schedule) (Schedule, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	next := time.Now().UTC().Add(time.Duration(sc.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	r, err := s.DB.ExecContext(ctx, `INSERT INTO schedules(profile_id,interval_minutes,enabled,next_run_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`, sc.ProfileID, sc.IntervalMinutes, boolInt(sc.Enabled), next, now, now)
	if err != nil {
		return sc, err
	}
	id, _ := r.LastInsertId()
	sc.ID = int(id)
	sc.CreatedAt = now
	sc.UpdatedAt = now
	sc.NextRunAt = &next
	return sc, nil
}
func (s *Store) UpdateSchedule(ctx context.Context, id int, sc Schedule) (*Schedule, error) {
	next := time.Now().UTC().Add(time.Duration(sc.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	_, err := s.DB.ExecContext(ctx, `UPDATE schedules SET interval_minutes=CASE WHEN ?>0 THEN ? ELSE interval_minutes END, enabled=?, next_run_at=CASE WHEN ?>0 THEN ? ELSE next_run_at END, updated_at=? WHERE id=?`, sc.IntervalMinutes, sc.IntervalMinutes, boolInt(sc.Enabled), sc.IntervalMinutes, next, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return nil, err
	}
	return s.GetSchedule(ctx, id)
}
func (s *Store) DeleteSchedule(ctx context.Context, id int) (bool, error) {
	r, err := s.DB.ExecContext(ctx, `DELETE FROM schedules WHERE id=?`, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}
func (s *Store) TouchScheduleRun(ctx context.Context, id int, next time.Time) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE schedules SET last_run_at=?, next_run_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), next.UTC().Format(time.RFC3339), id)
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
