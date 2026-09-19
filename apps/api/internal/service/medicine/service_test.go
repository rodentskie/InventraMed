package medicine

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/medicine"
	"apps/api/pkg/apperror"
)

type stubRepo struct {
	nameBatchTaken bool
	barcodeTaken   bool
	nameBatchErr   error
	barcodeErr     error
	createErr      error
	txErr          error

	calls      []string
	gotName    string
	gotBatch   *string
	gotBarcode string
	gotCreate  *domain.Medicine
}

func (s *stubRepo) ExistsByNameAndBatch(_ context.Context, name string, batch *string) (bool, error) {
	s.calls = append(s.calls, "name_batch")
	s.gotName = name
	s.gotBatch = batch

	return s.nameBatchTaken, s.nameBatchErr
}

func (s *stubRepo) ExistsByBarcode(_ context.Context, barcode string) (bool, error) {
	s.calls = append(s.calls, "barcode")
	s.gotBarcode = barcode

	return s.barcodeTaken, s.barcodeErr
}

func (s *stubRepo) Create(_ context.Context, m *domain.Medicine) (*domain.Medicine, error) {
	s.calls = append(s.calls, "create")
	s.gotCreate = m
	if s.createErr != nil {
		return nil, s.createErr
	}

	created := *m
	created.ID = "med-1"

	return &created, nil
}

func (s *stubRepo) Transaction(_ context.Context, fn func(tx medicine.Repository) error) error {
	if s.txErr != nil {
		return s.txErr
	}

	return fn(s)
}

func input() CreateInput {
	batch := "B-001"

	return CreateInput{
		Name:           "Paracetamol",
		Barcode:        "8901234567890",
		BatchNumber:    &batch,
		ExpirationDate: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		Quantity:       120,
		CreatedBy:      "user-1",
	}
}

func TestCreate_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Create(context.Background(), input())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != "med-1" {
		t.Errorf("id: got %q, want the repository's row", got.ID)
	}
	want := []string{"name_batch", "barcode", "create"}
	if len(repo.calls) != len(want) {
		t.Fatalf("calls: got %v, want %v", repo.calls, want)
	}
	for i := range want {
		if repo.calls[i] != want[i] {
			t.Fatalf("calls: got %v, want %v", repo.calls, want)
		}
	}
	if repo.gotName != "Paracetamol" || repo.gotBarcode != "8901234567890" {
		t.Errorf("checks got name %q barcode %q", repo.gotName, repo.gotBarcode)
	}
	c := repo.gotCreate
	if c.Name != "Paracetamol" || c.Barcode != "8901234567890" || c.Quantity != 120 {
		t.Errorf("created medicine: %+v", c)
	}
	if c.BatchNumber == nil || *c.BatchNumber != "B-001" {
		t.Errorf("batch number: got %v", c.BatchNumber)
	}
	if !c.ExpirationDate.Equal(input().ExpirationDate) {
		t.Errorf("expiration date: got %v", c.ExpirationDate)
	}
}

func TestCreate_CreatedByReachesRepository(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if _, err := svc.Create(context.Background(), input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotCreate.CreatedBy == nil || *repo.gotCreate.CreatedBy != "user-1" {
		t.Errorf("created_by: got %v, want user-1", repo.gotCreate.CreatedBy)
	}
}

func TestCreate_NilBatchIsPassedThrough(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())
	in := input()
	in.BatchNumber = nil

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotBatch != nil {
		t.Errorf("batch passed to the check: got %v, want nil", *repo.gotBatch)
	}
	if repo.gotCreate.BatchNumber != nil {
		t.Errorf("batch on the created medicine: got %v, want nil", *repo.gotCreate.BatchNumber)
	}
}

func TestCreate_Conflicts(t *testing.T) {
	tests := []struct {
		name      string
		repo      *stubRepo
		want      error
		wantCalls int
	}{
		{"name and batch taken", &stubRepo{nameBatchTaken: true}, apperror.ErrNameBatchExists, 1},
		{"barcode taken", &stubRepo{barcodeTaken: true}, apperror.ErrBarcodeExists, 2},
		{"both taken reports name and batch", &stubRepo{nameBatchTaken: true, barcodeTaken: true}, apperror.ErrNameBatchExists, 1},
		{"insert conflict on barcode", &stubRepo{createErr: apperror.ErrBarcodeExists}, apperror.ErrBarcodeExists, 3},
		{"insert conflict on name and batch", &stubRepo{createErr: apperror.ErrNameBatchExists}, apperror.ErrNameBatchExists, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, zap.NewNop())

			got, err := svc.Create(context.Background(), input())

			if got != nil {
				t.Errorf("expected no medicine, got %+v", got)
			}
			if err != tt.want {
				t.Errorf("error: got %v, want %v unchanged", err, tt.want)
			}
			if !errors.Is(err, apperror.ErrConflict) {
				t.Errorf("error should wrap ErrConflict: %v", err)
			}
			if len(tt.repo.calls) != tt.wantCalls {
				t.Errorf("repository calls: got %v, want %d", tt.repo.calls, tt.wantCalls)
			}
		})
	}
}

func TestCreate_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := map[string]*stubRepo{
		"name and batch check": {nameBatchErr: boom},
		"barcode check":        {barcodeErr: boom},
		"insert":               {createErr: boom},
		"transaction":          {txErr: boom},
	}

	for name, repo := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewService(repo, zap.NewNop())

			got, err := svc.Create(context.Background(), input())

			if got != nil {
				t.Errorf("expected no medicine, got %+v", got)
			}
			if !errors.Is(err, boom) {
				t.Errorf("error should wrap the repository error: %v", err)
			}
			if errors.Is(err, apperror.ErrConflict) {
				t.Error("a repository failure must not look like a conflict")
			}
		})
	}
}
