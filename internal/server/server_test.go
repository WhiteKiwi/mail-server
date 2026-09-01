package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/whitekiwi/mail-server/internal/config"
	"github.com/whitekiwi/mail-server/internal/delivery"
	mailtemplate "github.com/whitekiwi/mail-server/internal/template"
)

type fakeStore struct {
	reservation       delivery.Reservation
	reserveErr        error
	completed, failed string
}

func (*fakeStore) Ready(context.Context) error { return nil }
func (s *fakeStore) Reserve(context.Context, string, [32]byte, [32]byte, [32]byte, string, time.Time) (delivery.Reservation, error) {
	return s.reservation, s.reserveErr
}
func (s *fakeStore) Complete(_ context.Context, id string, _ time.Time) error {
	s.completed = id
	return nil
}
func (s *fakeStore) Fail(_ context.Context, id string, _ time.Time) error { s.failed = id; return nil }

type fakeMailer struct {
	message mailtemplate.Message
	err     error
}

func (m *fakeMailer) Send(_ context.Context, message mailtemplate.Message) error {
	m.message = message
	return m.err
}

func TestDeliveryAuthenticatesScopesAndAvoidsSecretLogs(t *testing.T) {
	store := &fakeStore{reservation: delivery.Reservation{ID: "eml_0123456789abcdef0123456789abcdef"}}
	mailer := &fakeMailer{}
	var logs bytes.Buffer
	app, _ := New(store, mailer, []config.Client{{ID: "c6s", Token: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", Templates: []string{mailtemplate.CerberusInvitation}, FromAddress: "no-reply@whitekiwi.link"}}, slog.New(slog.NewJSONHandler(&logs, nil)))
	body := `{"template":"cerberus.organization-invitation","recipient":"person@example.com","locale":"en","variables":{"invitationLink":"https://console.c6s.whitekiwi.link/invitations/accept/#token=secret-token-value-0123456789abcdef"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/deliveries", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	request.Header.Set("Idempotency-Key", "outbox-request-0001")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || store.completed == "" || mailer.message.Recipient != "person@example.com" || mailer.message.FromAddress != "no-reply@whitekiwi.link" {
		t.Fatalf("status=%d store=%#v mail=%#v body=%s", response.Code, store, mailer.message, response.Body.String())
	}
	logText, _ := io.ReadAll(&logs)
	if bytes.Contains(logText, []byte("person@example.com")) || bytes.Contains(logText, []byte("secret-token-value")) {
		t.Fatalf("private value leaked in logs: %s", logText)
	}
}

func TestDeliveryRejectsUnauthorizedAndTemplateConfusion(t *testing.T) {
	store := &fakeStore{}
	app, _ := New(store, &fakeMailer{}, []config.Client{{ID: "other", Token: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", Templates: []string{"other.template"}}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, token := range []string{"", "Bearer wrong", "Bearer abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"} {
		request := httptest.NewRequest(http.MethodPost, "/v1/deliveries", bytes.NewBufferString(`{"template":"cerberus.organization-invitation"}`))
		request.Header.Set("Authorization", token)
		request.Header.Set("Idempotency-Key", "outbox-request-0001")
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
			t.Fatalf("token %q status=%d", token, response.Code)
		}
	}
}
