package delivery

import "testing"

func TestDockerTransportPreservesOnlyTheRuntimeTarget(t *testing.T) {
	t.Setenv("MAIL_DATABASE_TRANSPORT", "docker-desktop")
	url := "postgres://mail_runtime:fixture@127.0.0.1:25432/mail?sslmode=disable"
	cfg, err := runtimePoolConfig(url)
	if err != nil || cfg.ConnConfig.Host != "host.docker.internal" || cfg.ConnConfig.Port != 25432 || cfg.ConnConfig.User != "mail_runtime" {
		t.Fatal("runtime transport failed")
	}
	for _, rejected := range []string{
		"postgres://mail_migration:fixture@127.0.0.1:25432/mail?sslmode=disable",
		"postgres://mail_owner:fixture@127.0.0.1:25432/mail?sslmode=disable",
		"postgres://mail_runtime:fixture@host.docker.internal:25432/mail?sslmode=disable",
		"postgres://mail_runtime:fixture@127.0.0.1:5432/mail?sslmode=disable",
		"postgres://mail_runtime:fixture@127.0.0.1:25432/postgres?sslmode=disable",
		"postgres://mail_runtime:fixture@127.0.0.1:25432/mail?sslmode=disable&role=mail_owner",
	} {
		if _, err := runtimePoolConfig(rejected); err == nil {
			t.Fatal("unqualified target accepted")
		}
	}
	t.Setenv("MAIL_DATABASE_TRANSPORT", "unknown")
	if _, err := runtimePoolConfig(url); err == nil {
		t.Fatal("unknown transport accepted")
	}
}
