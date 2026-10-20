package delivery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	mailtemplate "github.com/whitekiwi/mail-server/internal/template"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProviderFailureStageIsBounded(t *testing.T) {
	if got := ProviderFailureStage(providerFailure("authenticate")); got != "authenticate" {
		t.Fatalf("stage=%q", got)
	}
	if got := ProviderFailureStage(errors.New("smtp response with private context")); got != "unknown" {
		t.Fatalf("untrusted error stage=%q", got)
	}
}

func TestEventReferenceAllowsOnlyOpaqueDeliveryIdentity(t *testing.T) {
	for _, value := range []string{"eml_0123456789abcdef", "eml_retry_2"} {
		if !validEventReference(value) {
			t.Fatalf("expected valid event reference %q", value)
		}
	}
	for _, value := range []string{"", "eml_", "delivery-1", "eml_recipient@example.com", "eml_value\r\nBcc:test"} {
		if validEventReference(value) {
			t.Fatalf("expected invalid event reference %q", value)
		}
	}
}

func TestAcceptedDATAIsSuccessWhenCancellationClosesQUIT(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, peer := net.Pipe()
	defer peer.Close()
	_ = peer.SetDeadline(time.Now().Add(3 * time.Second))
	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(peer)
		reply := func(text string) error { _, err := fmt.Fprint(peer, text); return err }
		command := func(prefix, response string) error {
			line, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			if !strings.HasPrefix(line, prefix) {
				return fmt.Errorf("unexpected protocol stage")
			}
			return reply(response)
		}
		if err := reply("220 localhost ESMTP\r\n"); err != nil {
			serverDone <- err
			return
		}
		for _, step := range [][2]string{{"EHLO ", "250-localhost\r\n250 AUTH PLAIN\r\n"}, {"AUTH PLAIN", "235 authenticated\r\n"}, {"MAIL FROM:", "250 sender\r\n"}, {"RCPT TO:", "250 recipient\r\n"}, {"DATA", "354 continue\r\n"}} {
			if err := command(step[0], step[1]); err != nil {
				serverDone <- err
				return
			}
		}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}
			if line == ".\r\n" {
				break
			}
		}
		if err := reply("250 accepted\r\n"); err != nil {
			serverDone <- err
			return
		}
		cancel()
		_ = peer.Close()
		serverDone <- nil
	}()
	mailer := NewSMTPMailer(SMTPConfig{Host: "localhost", Port: 465, Username: "fixture", Password: "fixture", FromAddress: "no-reply@whitekiwi.link"})
	// Port465's TLS setup precedes sendConnected; a pipe models that completed transport.
	err := mailer.sendConnected(ctx, client, mailtemplate.Message{Recipient: "fixture@example.com", FromName: "Fixture", Subject: "Fixture", Text: "Fixture"})
	if err != nil {
		t.Fatal("known DATA acceptance was changed into a provider failure", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}
