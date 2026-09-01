package delivery

import (
	"errors"
	"testing"
)

func TestProviderFailureStageIsBounded(t *testing.T) {
	if got := ProviderFailureStage(providerFailure("authenticate")); got != "authenticate" {
		t.Fatalf("stage=%q", got)
	}
	if got := ProviderFailureStage(errors.New("smtp response with private context")); got != "unknown" {
		t.Fatalf("untrusted error stage=%q", got)
	}
}
