package config

import "testing"

func TestDockerListenerKeepsNativeConfigurationUnchanged(t *testing.T) {
	t.Setenv("MAIL_DATABASE_TRANSPORT", "docker-desktop")
	original := Config{ListenAddress: "127.0.0.1:8092"}
	container, err := runtimeTransport(original)
	if err != nil || container.ListenAddress != "0.0.0.0:8092" || original.ListenAddress != "127.0.0.1:8092" {
		t.Fatal("listener transport changed protected configuration")
	}
	if _, err := runtimeTransport(Config{ListenAddress: "127.0.0.1:8093"}); err == nil {
		t.Fatal("unknown listener accepted")
	}
	t.Setenv("MAIL_DATABASE_TRANSPORT", "unknown")
	if _, err := runtimeTransport(original); err == nil {
		t.Fatal("unknown transport accepted")
	}
}
