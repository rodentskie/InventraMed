package supplier

import (
	"context"
	"errors"
	"slices"
	"testing"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/internal/repository/supplier"
	"apps/api/pkg/apperror"
)

type stubRepo struct {
	lockMissing bool
	inPurchase  bool

	lockErr     error
	purchaseErr error
	listErr     error
	findErr     error
	createErr   error
	updateErr   error
	deleteErr   error

	listed []*domain.Supplier
	total  int64
	found  *domain.Supplier

	calls       []string
	gotCreate   *domain.Supplier
	gotUpdate   *domain.Supplier
	gotFilter   supplier.ListFilter
	gotFindID   string
	gotDeleteID string
}

func (s *stubRepo) Create(_ context.Context, sup *domain.Supplier) (*domain.Supplier, error) {
	s.calls = append(s.calls, "create")
	s.gotCreate = sup
	if s.createErr != nil {
		return nil, s.createErr
	}

	created := *sup
	created.ID = "sup-1"

	return &created, nil
}

func (s *stubRepo) List(_ context.Context, filter supplier.ListFilter) ([]*domain.Supplier, int64, error) {
	s.calls = append(s.calls, "list")
	s.gotFilter = filter

	return s.listed, s.total, s.listErr
}

func (s *stubRepo) FindByID(_ context.Context, id string) (*domain.Supplier, error) {
	s.calls = append(s.calls, "find")
	s.gotFindID = id

	return s.found, s.findErr
}

func (s *stubRepo) Update(_ context.Context, sup *domain.Supplier) error {
	s.calls = append(s.calls, "update")
	s.gotUpdate = sup

	return s.updateErr
}

func (s *stubRepo) LockByID(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "lock")

	return !s.lockMissing, s.lockErr
}

func (s *stubRepo) ExistsInPurchaseOrder(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "purchase_order")

	return s.inPurchase, s.purchaseErr
}

func (s *stubRepo) Delete(_ context.Context, id string) error {
	s.calls = append(s.calls, "delete")
	s.gotDeleteID = id

	return s.deleteErr
}

func (s *stubRepo) Transaction(_ context.Context, fn func(tx supplier.Repository) error) error {
	return fn(s)
}

func (s *stubRepo) called(name string) bool {
	return slices.Contains(s.calls, name)
}

func str(value string) *string {
	return &value
}

func input() CreateInput {
	return CreateInput{
		Name:        "Acme Pharma Distribution",
		ContactName: str("Jane Cruz"),
		Email:       str("jane@acmepharma.example"),
		Phone:       str("+63 917 123 4567"),
		Address:     str("123 Industrial Ave, Quezon City"),
	}
}

func TestCreate_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Create(context.Background(), input())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != "sup-1" {
		t.Errorf("id: got %q, want the repository's row", got.ID)
	}
	if !slices.Equal(repo.calls, []string{"create"}) {
		t.Errorf("calls: got %v, want only create", repo.calls)
	}
	c := repo.gotCreate
	if c.Name != "Acme Pharma Distribution" || *c.ContactName != "Jane Cruz" ||
		*c.Email != "jane@acmepharma.example" || *c.Phone != "+63 917 123 4567" ||
		*c.Address != "123 Industrial Ave, Quezon City" {
		t.Errorf("created supplier: %+v", c)
	}
}

func TestCreate_NilFieldsArePassedThrough(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if _, err := svc.Create(context.Background(), CreateInput{Name: "Acme"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := repo.gotCreate
	if c.ContactName != nil || c.Email != nil || c.Phone != nil || c.Address != nil {
		t.Errorf("optional fields: got %+v, want all nil", c)
	}
}

func TestCreate_RepositoryError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewService(&stubRepo{createErr: boom}, zap.NewNop())

	got, err := svc.Create(context.Background(), input())

	if got != nil {
		t.Errorf("expected no supplier, got %+v", got)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error: got %v, want it to wrap %v", err, boom)
	}
}

func TestList_PassesTheFilterThrough(t *testing.T) {
	listed := []*domain.Supplier{{ID: "sup-1"}, {ID: "sup-2"}}
	repo := &stubRepo{listed: listed, total: 12}
	svc := NewService(repo, zap.NewNop())
	filter := ListFilter{Limit: 10, Offset: 20, Name: "acme"}

	page, err := svc.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotFilter != filter {
		t.Errorf("filter: got %+v, want %+v", repo.gotFilter, filter)
	}
	if page.Total != 12 || !slices.Equal(page.Suppliers, listed) {
		t.Errorf("page: got %+v", page)
	}
}

func TestList_RepositoryError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewService(&stubRepo{listErr: boom}, zap.NewNop())

	page, err := svc.List(context.Background(), ListFilter{Limit: 20})

	if page != nil {
		t.Errorf("expected no page, got %+v", page)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error: got %v, want it to wrap %v", err, boom)
	}
}

func TestGetByID(t *testing.T) {
	boom := errors.New("boom")

	t.Run("found", func(t *testing.T) {
		want := &domain.Supplier{ID: "sup-1"}
		repo := &stubRepo{found: want}
		svc := NewService(repo, zap.NewNop())

		got, err := svc.GetByID(context.Background(), "sup-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != want {
			t.Errorf("supplier: got %+v, want %+v", got, want)
		}
		if repo.gotFindID != "sup-1" {
			t.Errorf("id: got %q", repo.gotFindID)
		}
	})

	t.Run("not found passes through unchanged", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: apperror.ErrNotFound}, zap.NewNop())

		got, err := svc.GetByID(context.Background(), "sup-1")

		if got != nil {
			t.Errorf("expected no supplier, got %+v", got)
		}
		if err != apperror.ErrNotFound {
			t.Errorf("error: got %v, want ErrNotFound unchanged", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: boom}, zap.NewNop())

		_, err := svc.GetByID(context.Background(), "sup-1")

		if !errors.Is(err, boom) {
			t.Errorf("error: got %v, want it to wrap %v", err, boom)
		}
	})
}

func TestUpdate_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if err := svc.Update(context.Background(), "sup-1", input()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !slices.Equal(repo.calls, []string{"update"}) {
		t.Errorf("calls: got %v, want only update", repo.calls)
	}
	u := repo.gotUpdate
	if u.ID != "sup-1" || u.Name != "Acme Pharma Distribution" || *u.ContactName != "Jane Cruz" ||
		*u.Email != "jane@acmepharma.example" || *u.Phone != "+63 917 123 4567" ||
		*u.Address != "123 Industrial Ave, Quezon City" {
		t.Errorf("updated supplier: %+v", u)
	}
}

func TestUpdate_ClearedFieldsAreNil(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if err := svc.Update(context.Background(), "sup-1", UpdateInput{Name: "Acme"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := repo.gotUpdate
	if u.ContactName != nil || u.Email != nil || u.Phone != nil || u.Address != nil {
		t.Errorf("optional fields: got %+v, want all nil", u)
	}
}

func TestUpdate_NotFoundPassesThrough(t *testing.T) {
	svc := NewService(&stubRepo{updateErr: apperror.ErrNotFound}, zap.NewNop())

	err := svc.Update(context.Background(), "sup-1", input())

	if err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
}

func TestUpdate_RepositoryError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewService(&stubRepo{updateErr: boom}, zap.NewNop())

	err := svc.Update(context.Background(), "sup-1", input())

	if !errors.Is(err, boom) {
		t.Errorf("error: got %v, want it to wrap %v", err, boom)
	}
}

func TestDelete_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if err := svc.Delete(context.Background(), "sup-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"lock", "purchase_order", "delete"}
	if !slices.Equal(repo.calls, want) {
		t.Errorf("calls: got %v, want %v", repo.calls, want)
	}
	if repo.gotDeleteID != "sup-1" {
		t.Errorf("deleted id: got %q", repo.gotDeleteID)
	}
}

func TestDelete_NotFoundSkipsThePurchaseOrderCheck(t *testing.T) {
	repo := &stubRepo{lockMissing: true}
	svc := NewService(repo, zap.NewNop())

	err := svc.Delete(context.Background(), "sup-1")

	if err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
	if repo.called("purchase_order") || repo.called("delete") {
		t.Errorf("calls: got %v, want nothing after the lock", repo.calls)
	}
}

func TestDelete_InPurchaseOrderIsNotDeleted(t *testing.T) {
	repo := &stubRepo{inPurchase: true}
	svc := NewService(repo, zap.NewNop())

	err := svc.Delete(context.Background(), "sup-1")

	if err != apperror.ErrSupplierInPurchaseOrder {
		t.Errorf("error: got %v, want ErrSupplierInPurchaseOrder unchanged", err)
	}
	if !errors.Is(err, apperror.ErrConflict) {
		t.Errorf("error should wrap ErrConflict, got %v", err)
	}
	if repo.called("delete") {
		t.Error("the supplier must not be deleted while a purchase order uses it")
	}
}

func TestDelete_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		repo *stubRepo
	}{
		{"lock", &stubRepo{lockErr: boom}},
		{"purchase order check", &stubRepo{purchaseErr: boom}},
		{"delete", &stubRepo{deleteErr: boom}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, zap.NewNop())

			err := svc.Delete(context.Background(), "sup-1")

			if !errors.Is(err, boom) {
				t.Errorf("error: got %v, want it to wrap %v", err, boom)
			}
		})
	}
}

func TestDelete_NotFoundFromDeletePassesThrough(t *testing.T) {
	svc := NewService(&stubRepo{deleteErr: apperror.ErrNotFound}, zap.NewNop())

	err := svc.Delete(context.Background(), "sup-1")

	if err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
}
