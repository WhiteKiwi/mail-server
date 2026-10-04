package delivery

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/whitekiwi/mail-server/internal/migrations"
)

// This test creates only its own database/roles in the disposable test cluster.
func TestRestrictedRuntimeSchemaAndDelivery(t *testing.T) {
	dsn := os.Getenv("MAIL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MAIL_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	db, owner, role := "mail_test_"+suffix, "mail_owner_"+suffix, "mail_runtime_"+suffix
	ident := func(s string) string { return pgx.Identifier{s}.Sanitize() }
	for _, sql := range []string{
		"CREATE ROLE " + ident(owner) + " NOLOGIN NOSUPERUSER NOCREATEROLE NOCREATEDB NOREPLICATION NOBYPASSRLS",
		"CREATE ROLE " + ident(role) + " LOGIN PASSWORD 'integration-only' NOSUPERUSER NOCREATEROLE NOCREATEDB NOREPLICATION NOBYPASSRLS",
		"CREATE DATABASE " + ident(db) + " OWNER " + ident(owner),
	} {
		if _, err := admin.pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, sql := range []string{"DROP DATABASE " + ident(db) + " WITH (FORCE)", "DROP ROLE " + ident(role), "DROP ROLE " + ident(owner)} {
			if _, err := admin.pool.Exec(ctx, sql); err != nil {
				t.Error(err)
			}
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db
	database, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.VerifySchema(ctx); err == nil {
		t.Fatal("empty schema was admitted")
	}
	tx, err := database.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+ident(owner)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, migrations.Initial); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"REVOKE ALL ON DATABASE " + ident(db) + " FROM PUBLIC",
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"GRANT CONNECT ON DATABASE " + ident(db) + " TO " + ident(role),
		"GRANT USAGE ON SCHEMA public TO " + ident(role),
		"GRANT SELECT, INSERT, UPDATE ON public.mail_deliveries TO " + ident(role),
	} {
		if _, err := database.pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	u.User = url.UserPassword(role, "integration-only")
	runtime, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	key, request, recipient := sha256.Sum256([]byte("fixture key")), sha256.Sum256([]byte("fixture request")), sha256.Sum256([]byte("fixture recipient"))
	r, err := runtime.Reserve(ctx, "c6s", key, request, recipient, "cerberus.organization-invitation", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reserve(ctx, "c6s", key, request, recipient, "cerberus.organization-invitation", now); !errors.Is(err, ErrInProgress) {
		t.Fatalf("pending replay = %v", err)
	}
	if err := runtime.Fail(ctx, r.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reserve(ctx, "c6s", key, request, recipient, "cerberus.organization-invitation", now); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Complete(ctx, r.ID, now); err != nil {
		t.Fatal(err)
	}
	if r, err := runtime.Reserve(ctx, "c6s", key, request, recipient, "cerberus.organization-invitation", now); err != nil || !r.Duplicate {
		t.Fatalf("delivered replay: duplicate=%v error=%v", r.Duplicate, err)
	}
	for _, sql := range []string{
		"CREATE TABLE public.forbidden(id integer)", "ALTER TABLE public.mail_deliveries ADD COLUMN forbidden text",
		"DELETE FROM public.mail_deliveries", "CREATE ROLE forbidden", "CREATE DATABASE forbidden", "SET ROLE " + ident(owner),
	} {
		_, err := runtime.pool.Exec(ctx, sql)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("expected insufficient_privilege for %q, got %v", sql, err)
		}
	}
	if _, err := database.pool.Exec(ctx, "DROP INDEX public.mail_deliveries_updated_idx"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.VerifySchema(ctx); err == nil {
		t.Fatal("missing index was admitted")
	}
}
