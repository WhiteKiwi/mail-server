package delivery

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	mailtemplate "github.com/whitekiwi/mail-server/internal/template"
)

type SMTPConfig struct {
	Host                            string
	Port                            int
	Username, Password, FromAddress string
	SESConfigurationSet             string
}
type SMTPMailer struct{ config SMTPConfig }

type ProviderError struct{ Stage string }

func (e *ProviderError) Error() string { return "mail provider failure: " + e.Stage }

func ProviderFailureStage(err error) string {
	var providerError *ProviderError
	if errors.As(err, &providerError) {
		return providerError.Stage
	}
	return "unknown"
}

func providerFailure(stage string) error { return &ProviderError{Stage: stage} }

func NewSMTPMailer(config SMTPConfig) *SMTPMailer { return &SMTPMailer{config: config} }

func (m *SMTPMailer) Send(ctx context.Context, message mailtemplate.Message) error {
	fromAddress := message.FromAddress
	if fromAddress == "" {
		fromAddress = m.config.FromAddress
	}
	address := net.JoinHostPort(m.config.Host, strconv.Itoa(m.config.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var connection net.Conn
	var err error
	if m.config.Port == 465 {
		connection, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12})
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return providerFailure("connect")
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(connection, m.config.Host)
	if err != nil {
		return providerFailure("start")
	}
	defer client.Close()
	if m.config.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return providerFailure("starttls_required")
		}
		if err := client.StartTLS(&tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return providerFailure("starttls")
		}
	}
	if err := client.Auth(smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)); err != nil {
		return providerFailure("authenticate")
	}
	if err := client.Mail(fromAddress); err != nil {
		return providerFailure("sender")
	}
	if err := client.Rcpt(message.Recipient); err != nil {
		return providerFailure("recipient")
	}
	data, err := client.Data()
	if err != nil {
		return providerFailure("data")
	}
	body := "From: " + message.FromName + " <" + fromAddress + ">\r\nTo: " + message.Recipient + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", message.Subject) + "\r\n"
	configurationSet := message.SESConfigurationSet
	if configurationSet == "" {
		configurationSet = m.config.SESConfigurationSet
	}
	if configurationSet != "" {
		body += "X-SES-CONFIGURATION-SET: " + configurationSet + "\r\n"
	}
	if message.EventReference != "" {
		if !validEventReference(message.EventReference) {
			_ = data.Close()
			return providerFailure("message")
		}
		body += "X-SES-MESSAGE-TAGS: whitekiwi_delivery_id=" + message.EventReference + "\r\n"
	}
	body += "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + message.Text
	if _, err := io.Copy(data, bufio.NewReader(strings.NewReader(body))); err != nil {
		_ = data.Close()
		return providerFailure("write")
	}
	if err := data.Close(); err != nil {
		return providerFailure("commit")
	}
	if err := client.Quit(); err != nil {
		return providerFailure("quit")
	}
	return nil
}

func validEventReference(value string) bool {
	if len(value) < 5 || len(value) > 64 || !strings.HasPrefix(value, "eml_") {
		return false
	}
	for _, candidate := range value[4:] {
		if (candidate >= 'a' && candidate <= 'z') || (candidate >= '0' && candidate <= '9') || candidate == '_' {
			continue
		}
		return false
	}
	return true
}
