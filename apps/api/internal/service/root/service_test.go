package root

import "testing"

func TestServiceGreeting(t *testing.T) {
	svc := NewService()

	if got := svc.Greeting(); got != "inventramed REST API" {
		t.Fatalf("expected greeting message, got %q", got)
	}
}
