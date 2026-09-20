package inventory

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/inventory"
	medicinerepo "apps/api/internal/repository/medicine"
	"apps/api/pkg/apperror"
)

var errBoom = errors.New("boom")

// stubRepo implements inventory.Repository.
type stubRepo struct {
	created *domain.InventoryEntry
	found   *domain.InventoryEntry
	listed  []*domain.InventoryEntry
	total   int64

	createErr error
	findErr   error
	listErr   error
	txErr     error

	gotFilter inventory.ListFilter
	gotFindID string

	// medicines controls what the transaction's medicine repository reports.
	medicines *stubMedicineRepo
}

func (s *stubRepo) Create(_ context.Context, entry *domain.InventoryEntry) (*domain.InventoryEntry, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}

	return s.created, nil
}

func (s *stubRepo) FindByID(_ context.Context, id string) (*domain.InventoryEntry, error) {
	s.gotFindID = id

	return s.found, s.findErr
}

func (s *stubRepo) List(_ context.Context, filter inventory.ListFilter) ([]*domain.InventoryEntry, int64, error) {
	s.gotFilter = filter

	return s.listed, s.total, s.listErr
}

func (s *stubRepo) Transaction(ctx context.Context, fn func(tx inventory.Repository, medicines medicinerepo.Repository) error) error {
	if s.txErr != nil {
		return s.txErr
	}

	return fn(s, s.medicines)
}

// stubMedicineRepo implements only what the inventory service needs from
// medicine.Repository; the rest panic if ever called.
type stubMedicineRepo struct {
	medicinerepo.Repository

	quantity int
	found    bool
	err      error

	gotID string
}

func (s *stubMedicineRepo) LockQuantityByID(_ context.Context, id string) (int, bool, error) {
	s.gotID = id

	return s.quantity, s.found, s.err
}

func newService(repo *stubRepo) Service {
	return NewService(repo, zap.NewNop())
}

func TestCreate_Success(t *testing.T) {
	entry := &domain.InventoryEntry{ID: "entry-1", MedicineID: "med-1"}
	repo := &stubRepo{created: entry, medicines: &stubMedicineRepo{quantity: 10, found: true}}
	svc := newService(repo)

	got, err := svc.Create(context.Background(), CreateInput{
		MedicineID: "med-1",
		Direction:  domain.DirectionAddition,
		Quantity:   5,
		Reason:     "count_adjustment",
		CountedBy:  "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != entry {
		t.Errorf("got %v, want %v", got, entry)
	}
	if repo.medicines.gotID != "med-1" {
		t.Errorf("medicine lock id: got %q, want %q", repo.medicines.gotID, "med-1")
	}
}

func TestCreate_MedicineNotFound(t *testing.T) {
	repo := &stubRepo{medicines: &stubMedicineRepo{found: false}}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{
		MedicineID: "missing",
		Direction:  domain.DirectionAddition,
		Quantity:   5,
	})
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestCreate_SubtractionExactStockSucceeds(t *testing.T) {
	entry := &domain.InventoryEntry{ID: "entry-1"}
	repo := &stubRepo{created: entry, medicines: &stubMedicineRepo{quantity: 5, found: true}}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{
		MedicineID: "med-1",
		Direction:  domain.DirectionSubtraction,
		Quantity:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreate_SubtractionInsufficientStock(t *testing.T) {
	repo := &stubRepo{medicines: &stubMedicineRepo{quantity: 4, found: true}}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{
		MedicineID: "med-1",
		Direction:  domain.DirectionSubtraction,
		Quantity:   5,
	})
	if !errors.Is(err, apperror.ErrInsufficientQuantity) {
		t.Fatalf("got %v, want ErrInsufficientQuantity", err)
	}
	if !errors.Is(err, apperror.ErrConflict) {
		t.Errorf("expected ErrInsufficientQuantity to wrap ErrConflict")
	}
}

func TestCreate_LockErrorPassesThroughWrapped(t *testing.T) {
	repo := &stubRepo{medicines: &stubMedicineRepo{err: errBoom}}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{MedicineID: "med-1", Direction: domain.DirectionAddition, Quantity: 1})
	if err == nil || errors.Is(err, apperror.ErrNotFound) || errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("expected a wrapped generic error, got %v", err)
	}
}

func TestCreate_InsertErrorPassesThroughWrapped(t *testing.T) {
	repo := &stubRepo{createErr: errBoom, medicines: &stubMedicineRepo{quantity: 10, found: true}}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{MedicineID: "med-1", Direction: domain.DirectionAddition, Quantity: 1})
	if err == nil || errors.Is(err, apperror.ErrNotFound) || errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("expected a wrapped generic error, got %v", err)
	}
}

func TestCreate_TransactionError(t *testing.T) {
	repo := &stubRepo{txErr: errBoom}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateInput{MedicineID: "med-1", Direction: domain.DirectionAddition, Quantity: 1})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestList(t *testing.T) {
	entries := []*domain.InventoryEntry{{ID: "entry-1"}}
	repo := &stubRepo{listed: entries, total: 1}
	svc := newService(repo)

	page, err := svc.List(context.Background(), ListFilter{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 1 || len(page.Entries) != 1 {
		t.Errorf("got %+v", page)
	}
	if repo.gotFilter.Limit != 20 {
		t.Errorf("filter not passed through: %+v", repo.gotFilter)
	}
}

func TestList_Error(t *testing.T) {
	repo := &stubRepo{listErr: errBoom}
	svc := newService(repo)

	if _, err := svc.List(context.Background(), ListFilter{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetByID(t *testing.T) {
	entry := &domain.InventoryEntry{ID: "entry-1"}
	repo := &stubRepo{found: entry}
	svc := newService(repo)

	got, err := svc.GetByID(context.Background(), "entry-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != entry {
		t.Errorf("got %v, want %v", got, entry)
	}
	if repo.gotFindID != "entry-1" {
		t.Errorf("id not passed through: %q", repo.gotFindID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &stubRepo{findErr: apperror.ErrNotFound}
	svc := newService(repo)

	_, err := svc.GetByID(context.Background(), "missing")
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestGetByID_Error(t *testing.T) {
	repo := &stubRepo{findErr: errBoom}
	svc := newService(repo)

	if _, err := svc.GetByID(context.Background(), "id-1"); err == nil {
		t.Fatal("expected an error")
	}
}
