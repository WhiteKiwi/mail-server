package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Exercise the actual main startup and signal path with an unavailable database.
// No database DDL, provider connection or production input is used.
func TestProcessStartsWithOfflineDatabase(t *testing.T) {
	if os.Getenv("MAIL_OFFLINE_TEST_CHILD") == "1" {
		os.Args = os.Args[:1]
		main()
		os.Exit(0)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	token := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	config := map[string]any{
		"listen_address": address, "database_url": "postgres://mail_runtime:fixture@127.0.0.1:1/mail?sslmode=disable&pool_max_conns=1",
		"smtp_host": "127.0.0.1", "smtp_port": 1, "smtp_username": "fixture", "smtp_password": "fixture",
		"from_address": "no-reply@whitekiwi.link",
		"clients":      []map[string]any{{"id": "c6s", "token": token, "templates": []string{"cerberus.beta-invitation"}}},
	}
	data, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "production.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessStartsWithOfflineDatabase$")
	cmd.Env = append(os.Environ(), "MAIL_OFFLINE_TEST_CHILD=1", "MAIL_CONFIG_FILE="+path, "MAIL_DATABASE_SCHEMA=app", "MAIL_DATABASE_TRANSPORT=native")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(done) }()
	defer func() { _ = cmd.Process.Kill(); <-done }()
	client := &http.Client{Timeout: 8 * time.Second}
	url := "http://" + address
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get(url + "/healthz")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != 204 {
				t.Fatal("startup health rejected")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("HTTP did not start with unavailable database")
		}
		time.Sleep(20 * time.Millisecond)
	}
	body := `{"template":"cerberus.beta-invitation","recipient":"fixture@example.com","locale":"en","variables":{"invitationLink":"https://console.c6s.whitekiwi.link/signup/"}}`
	request, _ := http.NewRequest("POST", url+"/v1/deliveries", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", "offline-process-0001")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("offline request did not finish within its bound")
	}
	_ = response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatalf("offline request status=%d", response.StatusCode)
	}
	response, err = client.Get(url + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatal("dependency failure changed health")
	}
	client.CloseIdleConnections()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		if waitErr != nil {
			t.Fatal("SIGTERM did not exit cleanly")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SIGTERM shutdown did not finish")
	}
}
