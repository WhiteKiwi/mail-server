package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProtectedJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "production.json")
	contents := `{
  "listen_address":"127.0.0.1:8092",
  "database_url":"postgres://mail:test@127.0.0.1:5432/mail?sslmode=disable",
  "clients":[{"id":"c6s","token":"abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG","templates":["cerberus.organization-invitation"]}],
  "smtp_host":"email-smtp.ap-northeast-2.amazonaws.com",
  "smtp_port":587,
  "smtp_username":"user",
  "smtp_password":"secret",
  "from_address":"no-reply@whitekiwi.link",
  "ses_configuration_set":"whitekiwi-transactional"
}`
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAIL_CONFIG_FILE", path)
	cfg, err := Load()
	if err != nil || cfg.ListenAddress != "127.0.0.1:8092" || cfg.SESConfigurationSet != "whitekiwi-transactional" {
		t.Fatalf("cfg=%#v err=%v", cfg, err)
	}
}

func TestLoadRejectsLooseOrUnknownConfig(t *testing.T) {
	for name, item := range map[string]struct {
		contents string
		mode     os.FileMode
	}{
		"loose":   {`{}`, 0o644},
		"unknown": {`{"unexpected":true}`, 0o640},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "production.json")
			if err := os.WriteFile(path, []byte(item.contents), item.mode); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MAIL_CONFIG_FILE", path)
			if _, err := Load(); err == nil {
				t.Fatal("unsafe configuration was accepted")
			}
		})
	}
}
