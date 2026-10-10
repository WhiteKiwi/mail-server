package delivery

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// The host supplies only a runtime DSN into a disposable pre-provisioned DB.
// No administrator credential or SMTP provider is needed by this test.
func TestProvisionedRuntimeDelivery(t *testing.T) {
	dsn := os.Getenv("MAIL_TEST_RUNTIME_DATABASE_URL")
	if dsn == "" {
		t.Skip("MAIL_TEST_RUNTIME_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var restricted bool
	err = store.pool.QueryRow(ctx, `SELECT NOT (rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls OR rolinherit)
		AND NOT EXISTS (SELECT FROM pg_auth_members WHERE member=r.oid)
		FROM pg_roles r WHERE rolname=current_user`).Scan(&restricted)
	if err != nil || !restricted {
		t.Fatalf("runtime principal is not restricted: %v", err)
	}
	if err := store.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	key := sha256.Sum256([]byte("native fixture " + now.String()))
	request, recipient := sha256.Sum256([]byte("fixture request")), sha256.Sum256([]byte("fixture recipient"))
	reserve := func(key, request [32]byte, at time.Time) (Reservation, error) {
		return store.Reserve(ctx, "c6s", key, request, recipient, "cerberus.organization-invitation", at)
	}
	r, err := reserve(key, request, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reserve(key, request, now); !errors.Is(err, ErrInProgress) {
		t.Fatalf("active reservation replay: %v", err)
	}
	changed := sha256.Sum256([]byte("changed request"))
	if _, err := reserve(key, changed, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed request replay: %v", err)
	}
	if err := store.Fail(ctx, r.ID, now); err != nil {
		t.Fatal(err)
	}
	retry, err := reserve(key, request, now)
	if err != nil || retry.ID != r.ID || retry.Duplicate {
		t.Fatalf("failed reservation retry: %v", err)
	}
	var attempts int
	if err := store.pool.QueryRow(ctx, "SELECT attempt_count FROM mail_deliveries WHERE id=$1", r.ID).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("retry attempts=%d, error=%v", attempts, err)
	}
	if err := store.Complete(ctx, r.ID, now); err != nil {
		t.Fatal(err)
	}
	duplicate, err := reserve(key, request, now)
	if err != nil || duplicate.ID != r.ID || !duplicate.Duplicate {
		t.Fatalf("completed reservation replay: %v", err)
	}
	staleKey := sha256.Sum256([]byte("stale " + now.String()))
	stale, err := reserve(staleKey, request, now.Add(-6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reserve(staleKey, request, now)
	if err != nil || recovered.ID != stale.ID {
		t.Fatalf("stale reservation retry: %v", err)
	}
	if _, err := store.pool.Exec(ctx, "UPDATE mail_deliveries SET attempt_count=20,state='failed' WHERE id=$1", stale.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reserve(staleKey, request, now); !errors.Is(err, ErrInProgress) {
		t.Fatalf("retry ceiling: %v", err)
	}
	for _, sql := range []string{
		"CREATE TABLE public.forbidden(id integer)",
		"ALTER TABLE public.mail_deliveries ADD COLUMN forbidden text",
		"DELETE FROM public.mail_deliveries", "CREATE ROLE forbidden", "CREATE DATABASE forbidden",
		"SET ROLE mail_owner", "SET ROLE mail_migration", "CREATE TEMP TABLE forbidden(id integer)",
	} {
		_, err := store.pool.Exec(ctx, sql)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("expected insufficient_privilege for %q, got %v", sql, err)
		}
	}
}
