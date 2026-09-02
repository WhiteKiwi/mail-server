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
