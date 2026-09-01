package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/mail"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Client struct {
	ID                  string   `json:"id"`
	Token               string   `json:"token"`
	Templates           []string `json:"templates"`
	FromAddress         string   `json:"from_address,omitempty"`
	SESConfigurationSet string   `json:"ses_configuration_set,omitempty"`
}

type Config struct {
	ListenAddress       string   `json:"listen_address"`
	DatabaseURL         string   `json:"database_url"`
	Clients             []Client `json:"clients"`
	SMTPHost            string   `json:"smtp_host"`
	SMTPPort            int      `json:"smtp_port"`
	SMTPUsername        string   `json:"smtp_username"`
	SMTPPassword        string   `json:"smtp_password"`
	FromAddress         string   `json:"from_address"`
	SESConfigurationSet string   `json:"ses_configuration_set"`
}

func Load() (Config, error) {
	if path := os.Getenv("MAIL_CONFIG_FILE"); path != "" {
		return loadFile(path)
	}
	return loadEnvironment()
}

func loadFile(path string) (Config, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o027 != 0 || info.Size() > 64<<10 {
		return Config{}, errors.New("MAIL_CONFIG_FILE is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, errors.New("MAIL_CONFIG_FILE is unavailable")
	}
	defer file.Close()
	var cfg Config
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, errors.New("MAIL_CONFIG_FILE is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("MAIL_CONFIG_FILE is invalid")
	}
	return validate(cfg)
}

func loadEnvironment() (Config, error) {
	port, err := strconv.Atoi(os.Getenv("MAIL_SMTP_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, errors.New("MAIL_SMTP_PORT is invalid")
	}
	var clients []Client
	decoder := json.NewDecoder(strings.NewReader(os.Getenv("MAIL_CLIENTS_JSON")))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&clients); err != nil || len(clients) == 0 {
		return Config{}, errors.New("MAIL_CLIENTS_JSON is invalid")
	}
	seen := map[string]bool{}
	for _, client := range clients {
		if client.ID == "" || len(client.Token) < 43 || len(client.Templates) == 0 || seen[client.ID] {
			return Config{}, errors.New("MAIL_CLIENTS_JSON contains an invalid client")
		}
		seen[client.ID] = true
	}
	listen := os.Getenv("MAIL_LISTEN_ADDRESS")
	if listen == "" {
		listen = "127.0.0.1:8092"
	}
	if _, _, err := net.SplitHostPort(listen); err != nil {
		return Config{}, errors.New("MAIL_LISTEN_ADDRESS is invalid")
	}
	config := Config{ListenAddress: listen, DatabaseURL: os.Getenv("MAIL_DATABASE_URL"), Clients: clients,
		SMTPHost: os.Getenv("MAIL_SMTP_HOST"), SMTPPort: port, SMTPUsername: os.Getenv("MAIL_SMTP_USERNAME"),
		SMTPPassword: os.Getenv("MAIL_SMTP_PASSWORD"), FromAddress: strings.ToLower(os.Getenv("MAIL_FROM_ADDRESS")),
		SESConfigurationSet: os.Getenv("MAIL_SES_CONFIGURATION_SET")}
	return validate(config)
}

var clientIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
var configurationSetPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validate(config Config) (Config, error) {
	if config.ListenAddress == "" {
		config.ListenAddress = "127.0.0.1:8092"
	}
	if _, _, err := net.SplitHostPort(config.ListenAddress); err != nil {
		return Config{}, errors.New("mail listen address is invalid")
	}
	if config.SMTPPort < 1 || config.SMTPPort > 65535 || config.DatabaseURL == "" || config.SMTPHost == "" ||
		config.SMTPUsername == "" || config.SMTPPassword == "" || strings.ContainsAny(config.SMTPHost, "\r\n") ||
		!validFromAddress(config.FromAddress) || strings.ContainsAny(config.FromAddress, "\r\n") ||
		(config.SESConfigurationSet != "" && !configurationSetPattern.MatchString(config.SESConfigurationSet)) {
		return Config{}, errors.New("mail runtime configuration is incomplete")
	}
	config.FromAddress = strings.ToLower(config.FromAddress)
	if len(config.Clients) == 0 {
		return Config{}, errors.New("mail clients are invalid")
	}
	seen := map[string]bool{}
	for index, client := range config.Clients {
		if !clientIDPattern.MatchString(client.ID) || len(client.Token) < 43 || len(client.Templates) == 0 || seen[client.ID] {
			return Config{}, errors.New("mail clients are invalid")
		}
		if client.FromAddress == "" {
			client.FromAddress = config.FromAddress
		}
		client.FromAddress = strings.ToLower(client.FromAddress)
		if !validFromAddress(client.FromAddress) || strings.ContainsAny(client.FromAddress, "\r\n") {
			return Config{}, errors.New("mail clients are invalid")
		}
		if client.SESConfigurationSet == "" {
			client.SESConfigurationSet = config.SESConfigurationSet
		}
		if client.SESConfigurationSet != "" && !configurationSetPattern.MatchString(client.SESConfigurationSet) {
			return Config{}, errors.New("mail clients are invalid")
		}
		config.Clients[index] = client
		seen[client.ID] = true
	}
	return config, nil
}

func validFromAddress(value string) bool {
	value = strings.ToLower(value)
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && (strings.HasSuffix(value, "@whitekiwi.link") || strings.HasSuffix(value, "@obsdog.ai"))
}

func (c Client) TokenDigest() [32]byte { return sha256.Sum256([]byte(c.Token)) }
