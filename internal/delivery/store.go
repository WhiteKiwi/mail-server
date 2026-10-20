package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("idempotency conflict")
var ErrInProgress = errors.New("delivery in progress")

type Store struct {
	pool   *pgxpool.Pool
	schema string
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	store, err := New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	connectContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := store.pool.Ping(connectContext); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

// New validates configuration without requiring the database to be online.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	poolConfig, err := runtimePoolConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid mail database configuration")
	}
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	if poolConfig.MaxConns > 5 {
		poolConfig.MaxConns = 5
	}
	schema := os.Getenv("MAIL_DATABASE_SCHEMA")
	selected := strings.ReplaceAll(poolConfig.ConnConfig.RuntimeParams["search_path"], " ", "")
	if schema == "" {
		if selected == "app,pg_catalog" {
			schema = "app"
		} else {
			schema = "public"
		}
	}
	if selected != "" && selected != schema+",pg_catalog" {
		return nil, errors.New("database schema and search_path differ")
	}
	if schema != "public" && schema != "app" {
		return nil, errors.New("invalid mail database schema")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, schema: schema}, nil
}

func runtimePoolConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("invalid mail database configuration")
	}
	switch os.Getenv("MAIL_DATABASE_TRANSPORT") {
	case "", "native":
		return cfg, nil
	case "docker-desktop":
		if cfg.ConnConfig.Host == "127.0.0.1" && cfg.ConnConfig.Port == 25432 && cfg.ConnConfig.Database == "mail" && cfg.ConnConfig.User == "mail_runtime" && cfg.ConnConfig.TLSConfig == nil && cfg.ConnConfig.RuntimeParams["role"] == "" {
			cfg.ConnConfig.Host = "host.docker.internal"
			cfg.ConnConfig.Fallbacks = nil
			return cfg, nil
		}
	}
	return nil, errors.New("mail database transport rejected")
}

func (s *Store) Close()                          { s.pool.Close() }
func (s *Store) Ready(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Migrate(ctx context.Context, sql string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SET LOCAL search_path = "+pgx.Identifier{s.schema}.Sanitize()+", pg_catalog"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) table() string { return pgx.Identifier{s.schema, "mail_deliveries"}.Sanitize() }

type Reservation struct {
	ID        string
	Duplicate bool
}

func (s *Store) Reserve(ctx context.Context, client string, idempotency, request, recipient [32]byte, template string, now time.Time) (Reservation, error) {
	// Read-only admission precedes every reservation; outages fail requests, not health.
	if err := s.VerifySchema(ctx); err != nil {
		return Reservation{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)
	var id, state string
	var stored []byte
	var updated time.Time
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,state,request_digest,updated_at FROM %s WHERE client_id=$1 AND idempotency_digest=$2 FOR UPDATE`, s.table()), client, idempotency[:]).Scan(&id, &state, &stored, &updated)
	if err == nil {
		if len(stored) != 32 || !equal(stored, request[:]) {
			return Reservation{}, ErrConflict
		}
		if state == "delivered" {
			return Reservation{ID: id, Duplicate: true}, tx.Commit(ctx)
		}
		if state == "sending" && updated.After(now.Add(-5*time.Minute)) {
			return Reservation{}, ErrInProgress
		}
		command, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET state='sending',attempt_count=attempt_count+1,updated_at=$2,delivered_at=NULL WHERE id=$1 AND attempt_count<20`, s.table()), id, now)
		if err != nil || command.RowsAffected() != 1 {
			return Reservation{}, ErrInProgress
		}
		return Reservation{ID: id}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Reservation{}, err
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return Reservation{}, err
	}
	id = "eml_" + hex.EncodeToString(bytes)
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s(id,client_id,idempotency_digest,request_digest,recipient_digest,template_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'sending',$7,$7)`, s.table()), id, client, idempotency[:], request[:], recipient[:], template, now)
	if err != nil {
		return Reservation{}, err
	}
	return Reservation{ID: id}, tx.Commit(ctx)
}

func (s *Store) Complete(ctx context.Context, id string, now time.Time) error {
	command, err := s.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s SET state='delivered',updated_at=$2,delivered_at=$2 WHERE id=$1 AND state='sending'`, s.table()), id, now)
	if err == nil && command.RowsAffected() != 1 {
		return errors.New("delivery reservation is unavailable")
	}
	return err
}
func (s *Store) Fail(ctx context.Context, id string, now time.Time) error {
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s SET state='failed',updated_at=$2,delivered_at=NULL WHERE id=$1 AND state='sending'`, s.table()), id, now)
	return err
}
func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
