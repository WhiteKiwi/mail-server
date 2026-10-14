package main

import (
	"context"
	"testing"
)

func TestDockerTransportRejectsExplicitMigration(t *testing.T) {
	t.Setenv("MAIL_DATABASE_TRANSPORT", "docker-desktop")
	if err := migrate(context.Background(), "postgres://mail_runtime:fixture@127.0.0.1:25432/mail?sslmode=disable"); err == nil {
		t.Fatal("Docker migration was accepted")
	}
}
