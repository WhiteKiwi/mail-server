package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("idempotency conflict")
var ErrInProgress = errors.New("delivery in progress")

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close()                          { s.pool.Close() }
func (s *Store) Ready(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Migrate(ctx context.Context, sql string) error {
	_, err := s.pool.Exec(ctx, sql)
	return err
}

type Reservation struct {
	ID        string
	Duplicate bool
}

func (s *Store) Reserve(ctx context.Context, client string, idempotency, request, recipient [32]byte, template string, now time.Time) (Reservation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Reservation{}, err
	}
	defer tx.Rollback(ctx)
	var id, state string
	var stored []byte
	var updated time.Time
	err = tx.QueryRow(ctx, `SELECT id,state,request_digest,updated_at FROM mail_deliveries WHERE client_id=$1 AND idempotency_digest=$2 FOR UPDATE`, client, idempotency[:]).Scan(&id, &state, &stored, &updated)
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
		command, err := tx.Exec(ctx, `UPDATE mail_deliveries SET state='sending',attempt_count=attempt_count+1,updated_at=$2,delivered_at=NULL WHERE id=$1 AND attempt_count<20`, id, now)
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
	_, err = tx.Exec(ctx, `INSERT INTO mail_deliveries(id,client_id,idempotency_digest,request_digest,recipient_digest,template_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'sending',$7,$7)`, id, client, idempotency[:], request[:], recipient[:], template, now)
	if err != nil {
		return Reservation{}, err
	}
	return Reservation{ID: id}, tx.Commit(ctx)
}

func (s *Store) Complete(ctx context.Context, id string, now time.Time) error {
	command, err := s.pool.Exec(ctx, `UPDATE mail_deliveries SET state='delivered',updated_at=$2,delivered_at=$2 WHERE id=$1 AND state='sending'`, id, now)
	if err == nil && command.RowsAffected() != 1 {
		return errors.New("delivery reservation is unavailable")
	}
	return err
}
func (s *Store) Fail(ctx context.Context, id string, now time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE mail_deliveries SET state='failed',updated_at=$2,delivered_at=NULL WHERE id=$1 AND state='sending'`, id, now)
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
