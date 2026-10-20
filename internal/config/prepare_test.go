package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareFreezesProjectionAndPreservesClientPolicy(t *testing.T) {
	root := t.TempDir()
	pg := filepath.Join(root, "pg")
	os.Mkdir(pg, 0700)
	for key, value := range map[string]string{"host": "postgres.fixture.internal", "port": "5432", "database": "mail", "username": "mail_runtime", "password": "fixture:p@ss", "sslmode": "disable"} {
		if err := os.WriteFile(filepath.Join(pg, key), []byte(value), 0400); err != nil {
			t.Fatal(err)
		}
	}
	original := Config{ListenAddress: "127.0.0.1:8092", DatabaseURL: "postgres://mail_runtime:old@127.0.0.1:25432/mail", SMTPHost: "smtp.fixture.invalid", SMTPPort: 587, SMTPUsername: "fixture", SMTPPassword: "fixture-secret", FromAddress: "no-reply@whitekiwi.link", Clients: []Client{{ID: "c6s", Token: strings.Repeat("c", 43), Templates: []string{"cerberus.beta-invitation"}}}}
	data, _ := json.Marshal(original)
	source := filepath.Join(root, "source.json")
	os.WriteFile(source, data, 0400)
	projection := filepath.Join(root, "projected.json")
	if err := os.Symlink(source, projection); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "private", "production.json")
	if err := PrepareRuntime(projection, pg, out); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Clients[0].Token != original.Clients[0].Token || cfg.SMTPPassword != original.SMTPPassword || cfg.ListenAddress != "0.0.0.0:8092" || !strings.Contains(cfg.DatabaseURL, "pool_max_conns=5") {
		t.Fatal("policy changed or transport not applied")
	}
	info, _ := os.Lstat(out)
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0400 {
		t.Fatal("prepared input is not regular0400")
	}
	if err := PrepareRuntime(projection, pg, out); err == nil {
		t.Fatal("existing prepared file overwritten")
	}
	if err := os.Chmod(filepath.Join(pg, "username"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pg, "username"), []byte("mail_migration"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := PrepareRuntime(source, pg, filepath.Join(root, "other", "production.json")); err == nil {
		t.Fatal("runtime accepted migration principal")
	}
}

func TestMigrationInputContainsNoProviderCredentials(t *testing.T) {
	root := t.TempDir()
	pg := filepath.Join(root, "pg")
	os.Mkdir(pg, 0700)
	for key, value := range map[string]string{"host": "postgres.fixture.internal", "port": "5432", "database": "mail", "username": "mail_migration", "password": "fixture", "sslmode": "disable"} {
		os.WriteFile(filepath.Join(pg, key), []byte(value), 0400)
	}
	out := filepath.Join(root, "private", "migration.url")
	if err := PrepareMigration(pg, out); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAIL_MIGRATION_DATABASE_URL_FILE", out)
	u, err := MigrationDatabaseURL()
	if err != nil || !strings.Contains(u, "role=mail_owner") || !strings.Contains(u, "pool_max_conns=1") {
		t.Fatal("migration role/input differs")
	}
}
