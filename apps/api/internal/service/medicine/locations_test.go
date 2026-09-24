package medicine

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/pkg/apperror"
)

// stubSettings returns a canned threshold, 30 days unless set.
type stubSettings struct {
	threshold int
	err       error
	called    bool
}

func (s *stubSettings) Get(_ context.Context) (*domain.Settings, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	if s.threshold == 0 {
		return &domain.Settings{WarningThresholdDays: 30}, nil
	}

	return &domain.Settings{WarningThresholdDays: s.threshold}, nil
}

func placedAt(location int, expiration time.Time) *domain.Medicine {
	return &domain.Medicine{ID: "med", Location: &location, ExpirationDate: expiration}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestExpirationStatus(t *testing.T) {
	now := time.Date(2026, 9, 24, 23, 59, 0, 0, time.UTC)

	tests := []struct {
		name       string
		expiration time.Time
		want       string
	}{
		{"expired yesterday", date(2026, 9, 23), domain.ExpirationStatusExpire},
		{"expires today", date(2026, 9, 24), domain.ExpirationStatusNear},
		{"on the threshold", date(2026, 10, 24), domain.ExpirationStatusNear},
		{"one day past the threshold", date(2026, 10, 25), domain.ExpirationStatusGood},
		{"far away", date(2028, 1, 1), domain.ExpirationStatusGood},
		{"long expired", date(2020, 1, 1), domain.ExpirationStatusExpire},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expirationStatus(tt.expiration, 30, now); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpirationStatus_UsesTheUTCDay(t *testing.T) {
	// 2026-09-25 01:00 in UTC+8 is still 2026-09-24 in UTC.
	now := time.Date(2026, 9, 25, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	if got := expirationStatus(date(2026, 9, 24), 30, now); got != domain.ExpirationStatusNear {
		t.Errorf("got %q, want %q", got, domain.ExpirationStatusNear)
	}
}

func TestExpirationStatus_UsesTheThreshold(t *testing.T) {
	now := date(2026, 9, 24)
	expiration := date(2026, 10, 4) // 10 days away

	if got := expirationStatus(expiration, 7, now); got != domain.ExpirationStatusGood {
		t.Errorf("threshold 7: got %q, want good", got)
	}
	if got := expirationStatus(expiration, 10, now); got != domain.ExpirationStatusNear {
		t.Errorf("threshold 10: got %q, want near", got)
	}
}

func TestLocations(t *testing.T) {
	repo := &stubRepo{placed: []*domain.Medicine{
		placedAt(2, date(2026, 9, 1)),
		placedAt(7, date(2026, 10, 4)),
		placedAt(12, date(2027, 1, 1)),
		{ID: "unplaced", ExpirationDate: date(2026, 9, 1)},
	}}
	settings := &stubSettings{threshold: 14}
	svc := NewService(repo, settings, zap.NewNop()).(*service)
	svc.now = func() time.Time { return date(2026, 9, 24) }

	got, err := svc.Locations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []LocationStatus{
		{Location: 2, Status: domain.ExpirationStatusExpire},
		{Location: 7, Status: domain.ExpirationStatusNear},
		{Location: 12, Status: domain.ExpirationStatusGood},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if !settings.called {
		t.Error("settings were not read")
	}
}

func TestLocations_EmptyTray(t *testing.T) {
	svc := NewService(&stubRepo{}, &stubSettings{}, zap.NewNop())

	got, err := svc.Locations(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty, non-nil slice", got)
	}
}

func TestLocations_SettingsErrors(t *testing.T) {
	for _, cause := range []error{apperror.ErrNotFound, errors.New("connection reset")} {
		repo := &stubRepo{}
		svc := NewService(repo, &stubSettings{err: cause}, zap.NewNop())

		_, err := svc.Locations(context.Background())
		if err == nil {
			t.Fatalf("%v: expected an error", cause)
		}
		if errors.Is(err, apperror.ErrNotFound) {
			t.Errorf("%v: missing settings must not surface as a missing medicine", cause)
		}
		if repo.called("list_placed") {
			t.Errorf("%v: medicines listed after the settings failed", cause)
		}
	}
}

func TestLocations_RepositoryError(t *testing.T) {
	boom := errors.New("connection reset")
	svc := NewService(&stubRepo{placedErr: boom}, &stubSettings{}, zap.NewNop())

	_, err := svc.Locations(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want it to wrap %v", err, boom)
	}
}
