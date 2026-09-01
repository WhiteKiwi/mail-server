package server

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/whitekiwi/mail-server/internal/config"
	"github.com/whitekiwi/mail-server/internal/delivery"
	mailtemplate "github.com/whitekiwi/mail-server/internal/template"
)

type Store interface {
	Ready(context.Context) error
	Reserve(context.Context, string, [32]byte, [32]byte, [32]byte, string, time.Time) (delivery.Reservation, error)
	Complete(context.Context, string, time.Time) error
	Fail(context.Context, string, time.Time) error
}
type Mailer interface {
	Send(context.Context, mailtemplate.Message) error
}
type client struct {
	id        string
	token     [32]byte
	templates map[string]bool
	from      string
}
type Server struct {
	store   Store
	mailer  Mailer
	clients []client
	logger  *slog.Logger
	now     func() time.Time
}

func New(store Store, mailer Mailer, clients []config.Client, logger *slog.Logger) (*Server, error) {
	if store == nil || mailer == nil || logger == nil {
		return nil, errors.New("server dependencies are incomplete")
	}
	items := make([]client, 0, len(clients))
	for _, item := range clients {
		templates := map[string]bool{}
		for _, name := range item.Templates {
			templates[name] = true
		}
		items = append(items, client{id: item.ID, token: item.TokenDigest(), templates: templates, from: item.FromAddress})
	}
	return &Server{store: store, mailer: mailer, clients: items, logger: logger, now: time.Now}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /v1/deliveries", s.deliver)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if s.store.Ready(ctx) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deliver(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.authenticate(r.Header.Get("Authorization"))
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	idempotency := r.Header.Get("Idempotency-Key")
	if len(idempotency) < 16 || len(idempotency) > 128 || strings.ContainsAny(idempotency, "\r\n") {
		http.Error(w, "invalid idempotency key", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var request mailtemplate.Request
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !actor.templates[request.Template] {
		http.Error(w, "template is not allowed", http.StatusForbidden)
		return
	}
	message, err := mailtemplate.Render(request)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	message.FromAddress = actor.from
	canonical, _ := json.Marshal(request)
	requestDigest := sha256.Sum256(canonical)
	recipientDigest := sha256.Sum256([]byte(strings.ToLower(message.Recipient)))
	idempotencyDigest := sha256.Sum256([]byte(idempotency))
	now := s.now().UTC()
	reservation, err := s.store.Reserve(r.Context(), actor.id, idempotencyDigest, requestDigest, recipientDigest, request.Template, now)
	if errors.Is(err, delivery.ErrConflict) {
		http.Error(w, "idempotency conflict", http.StatusConflict)
		return
	}
	if errors.Is(err, delivery.ErrInProgress) {
		http.Error(w, "delivery is in progress", http.StatusConflict)
		return
	}
	if err != nil {
		s.logger.Error("reserve mail delivery failed")
		http.Error(w, "delivery unavailable", http.StatusServiceUnavailable)
		return
	}
	if !reservation.Duplicate {
		if err := s.mailer.Send(r.Context(), message); err != nil {
			_ = s.store.Fail(r.Context(), reservation.ID, s.now().UTC())
			s.logger.Warn("mail provider delivery failed", "delivery_id", reservation.ID)
			http.Error(w, "delivery unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := s.store.Complete(r.Context(), reservation.ID, s.now().UTC()); err != nil {
			s.logger.Error("complete mail delivery failed", "delivery_id", reservation.ID)
			http.Error(w, "delivery state unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{"deliveryId": reservation.ID, "state": map[bool]string{true: "duplicate", false: "accepted"}[reservation.Duplicate]})
}

func (s *Server) authenticate(header string) (client, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return client{}, false
	}
	digest := sha256.Sum256([]byte(strings.TrimPrefix(header, prefix)))
	match := -1
	for index, item := range s.clients {
		if subtle.ConstantTimeCompare(digest[:], item.token[:]) == 1 {
			match = index
		}
	}
	if match < 0 {
		return client{}, false
	}
	return s.clients[match], true
}
