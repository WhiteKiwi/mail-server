package delivery

import (
	"context"
	"testing"
)

func TestLazyPoolAndSchemaSelectionDoNotConnect(t *testing.T) {
	t.Setenv("MAIL_DATABASE_SCHEMA", "app")
	s, err := New(context.Background(), "postgres://mail_runtime:fixture@127.0.0.1:1/mail?sslmode=disable&pool_max_conns=40")
	if err != nil {
		t.Fatal("offline database prevented pool construction")
	}
	defer s.Close()
	if s.table() != `"app"."mail_deliveries"` || s.pool.Config().MaxConns != 5 {
		t.Fatal("schema/pool boundary differs")
	}
	t.Setenv("MAIL_DATABASE_SCHEMA", "app; DROP SCHEMA public")
	if _, err := New(context.Background(), "postgres://mail_runtime:fixture@127.0.0.1:1/mail?sslmode=disable"); err == nil {
		t.Fatal("unknown schema accepted")
	}
}

func TestPreparedSearchPathSelectsSameMigrationAndRuntimeSchema(t *testing.T) {
	t.Setenv("MAIL_DATABASE_SCHEMA", "")
	store, err := New(context.Background(), "postgres://mail_migration:fixture@127.0.0.1:1/mail?sslmode=disable&search_path=app,pg_catalog&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if store.table() != `"app"."mail_deliveries"` {
		t.Fatal("prepared migration silently selected public")
	}
	t.Setenv("MAIL_DATABASE_SCHEMA", "public")
	if _, err := New(context.Background(), "postgres://mail_migration:fixture@127.0.0.1:1/mail?sslmode=disable&search_path=app,pg_catalog"); err == nil {
		t.Fatal("conflicting schema configuration accepted")
	}
}
