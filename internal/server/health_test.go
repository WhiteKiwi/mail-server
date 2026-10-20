package server

import (
	"bytes"
	"context"
	"errors"
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

func TestHealthDoesNotUseUnavailableDatabase(t *testing.T) {
	store := &fakeStore{reserveErr: errors.New("offline")}
	mailer := &fakeMailer{}
	app, _ := New(store, mailer, []config.Client{{ID: "c6s", Token: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", Templates: []string{"cerberus.beta-invitation"}}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, path := range []string{"/healthz", "/readyz"} {
		w := httptest.NewRecorder()
		app.Handler().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		want := http.StatusNoContent
		if path == "/readyz" {
			want = http.StatusNotFound
		}
		if w.Code != want {
			t.Fatalf("path=%s status=%d", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/v1/deliveries", bytes.NewBufferString(`{"template":"cerberus.beta-invitation","recipient":"fixture@example.com","locale":"en","variables":{"invitationLink":"https://console.c6s.whitekiwi.link/signup/"}}`))
	r.Header.Set("Authorization", "Bearer abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	r.Header.Set("Idempotency-Key", "fixture-offline-0001")
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if w.Code != 503 || mailer.message.Recipient != "" {
		t.Fatal("offline database reached provider")
	}
	w = httptest.NewRecorder()
	app.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 204 {
		t.Fatal("dependency failure changed health")
	}
}

type cancelMailer struct{ cancel context.CancelFunc }

func (m cancelMailer) Send(context.Context, mailtemplate.Message) error { m.cancel(); return nil }

// A client disconnect after known SMTP acceptance must not cancel finalization.
type finalizationStore struct {
	fakeStore
	completedContext error
}

func (s *finalizationStore) Complete(ctx context.Context, id string, now time.Time) error {
	s.completedContext = ctx.Err()
	return s.fakeStore.Complete(ctx, id, now)
}

func TestKnownAcceptanceFinalizesAfterClientCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &finalizationStore{fakeStore: fakeStore{reservation: delivery.Reservation{ID: "eml_0123456789abcdef0123456789abcdef"}}}
	app, _ := New(store, cancelMailer{cancel}, []config.Client{{ID: "c6s", Token: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", Templates: []string{"cerberus.beta-invitation"}}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := httptest.NewRequest("POST", "/v1/deliveries", bytes.NewBufferString(`{"template":"cerberus.beta-invitation","recipient":"fixture@example.com","locale":"en","variables":{"invitationLink":"https://console.c6s.whitekiwi.link/signup/"}}`)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	r.Header.Set("Idempotency-Key", "fixture-canceled-0001")
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if store.completedContext != nil || store.completed == "" || w.Code != 202 {
		t.Fatal("client cancellation prevented recording known acceptance")
	}
}
