package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Projection inputs are trusted, read-only Kubernetes Secret mounts. Copy the
// selected bytes into an owned regular file; retain the runtime no-symlink check.
func projected(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("projected input unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, errors.New("projected input rejected")
	}
	data, err := io.ReadAll(io.LimitReader(f, 64<<10+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errors.New("projected input rejected")
	}
	return data, nil
}

func databaseInput(directory string, migration bool) (string, error) {
	values := map[string]string{}
	for _, key := range []string{"host", "port", "database", "username", "password", "sslmode"} {
		data, err := projected(filepath.Join(directory, key))
		if err != nil {
			return "", err
		}
		values[key] = string(data)
		if values[key] == "" || strings.ContainsAny(values[key], "\r\n\x00") {
			return "", errors.New("database input rejected")
		}
	}
	user, pool := "mail_runtime", "5"
	if migration {
		user, pool = "mail_migration", "1"
	}
	if values["database"] != "mail" || values["username"] != user || values["port"] != "5432" || values["sslmode"] != "disable" {
		return "", errors.New("database identity rejected")
	}
	if strings.ContainsAny(values["host"], "/@?# ") {
		return "", errors.New("database host rejected")
	}
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(values["host"], values["port"]), Path: "/mail", User: url.UserPassword(user, values["password"])}
	query := url.Values{"sslmode": {"disable"}, "pool_max_conns": {pool}, "search_path": {"app,pg_catalog"}}
	if migration {
		query.Set("role", "mail_owner")
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func writePrepared(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.Mkdir(parent, 0o700); err != nil && !os.IsExist(err) {
		return errors.New("prepared directory unavailable")
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("prepared directory rejected")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err != nil {
		return errors.New("prepared output already exists or is unavailable")
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return errors.New("prepare output failed")
	}
	return f.Sync()
}

func PrepareRuntime(source, postgresDirectory, destination string) error {
	data, err := projected(source)
	if err != nil {
		return err
	}
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return errors.New("source configuration rejected")
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		return errors.New("source configuration rejected")
	}
	// Validate the original provider/client policy before changing only transport.
	if _, err := validate(cfg); err != nil {
		return errors.New("source configuration rejected")
	}
	cfg.DatabaseURL, err = databaseInput(postgresDirectory, false)
	if err != nil {
		return err
	}
	cfg.ListenAddress = "0.0.0.0:8092"
	if _, err := validate(cfg); err != nil {
		return errors.New("prepared configuration rejected")
	}
	data, err = json.Marshal(cfg)
	if err != nil {
		return errors.New("prepared configuration rejected")
	}
	return writePrepared(destination, append(data, '\n'))
}

func PrepareMigration(postgresDirectory, destination string) error {
	value, err := databaseInput(postgresDirectory, true)
	if err != nil {
		return err
	}
	return writePrepared(destination, []byte(value))
}

func MigrationDatabaseURL() (string, error) {
	if path := os.Getenv("MAIL_MIGRATION_DATABASE_URL_FILE"); path != "" {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 8192 {
			return "", errors.New("migration input rejected")
		}
		data, err := os.ReadFile(path)
		if err != nil || strings.TrimSpace(string(data)) == "" {
			return "", errors.New("migration input unavailable")
		}
		return string(data), nil
	}
	return os.Getenv("MAIL_MIGRATION_DATABASE_URL"), nil
}
