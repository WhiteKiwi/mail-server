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

func NewSMTPMailer(config SMTPConfig) *SMTPMailer { return &SMTPMailer{config: config} }

func (m *SMTPMailer) Send(ctx context.Context, message mailtemplate.Message) error {
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
		return errors.New("connect SMTP provider")
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(connection, m.config.Host)
	if err != nil {
		return errors.New("start SMTP provider")
	}
	defer client.Close()
	if m.config.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP provider requires STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return errors.New("secure SMTP provider")
		}
	}
	if err := client.Auth(smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)); err != nil {
		return errors.New("authenticate SMTP provider")
	}
	if err := client.Mail(m.config.FromAddress); err != nil {
		return errors.New("set sender")
	}
	if err := client.Rcpt(message.Recipient); err != nil {
		return errors.New("set recipient")
	}
	data, err := client.Data()
	if err != nil {
		return errors.New("open message")
	}
	body := "From: " + message.FromName + " <" + m.config.FromAddress + ">\r\nTo: " + message.Recipient + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", message.Subject) + "\r\n"
	if m.config.SESConfigurationSet != "" {
		body += "X-SES-CONFIGURATION-SET: " + m.config.SESConfigurationSet + "\r\n"
	}
	body += "MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + message.Text
	if _, err := io.Copy(data, bufio.NewReader(strings.NewReader(body))); err != nil {
		_ = data.Close()
		return errors.New("write message")
	}
	if err := data.Close(); err != nil {
		return errors.New("finish message")
	}
	if err := client.Quit(); err != nil {
		return errors.New("finish SMTP provider session")
	}
	return nil
}
