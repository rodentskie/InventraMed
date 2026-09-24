package medicine

import (
	"context"
	"errors"
	"slices"
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
	locationTaken  bool
	idMissing      bool
	lockMissing    bool
	inPurchase     bool

	nameBatchErr error
	barcodeErr   error
	locationErr  error
	existsErr    error
	lockErr      error
	purchaseErr  error
	listErr      error
	findErr      error
	createErr    error
	updateErr    error
	deleteErr    error
	txErr        error

	listed []*domain.Medicine
	total  int64
	found  *domain.Medicine

	calls          []string
	gotName        string
	gotBatch       *string
	gotBarcode     string
	gotLocation    int
	gotExclude     []string
	gotCreate      *domain.Medicine
	gotUpdate      *domain.Medicine
	gotDeleteID    string
	gotFilter      medicine.ListFilter
	gotFindBarcode string
}

func (s *stubRepo) ExistsByNameAndBatch(_ context.Context, name string, batch *string, excludeID string) (bool, error) {
	s.calls = append(s.calls, "name_batch")
	s.gotName = name
	s.gotBatch = batch
	s.gotExclude = append(s.gotExclude, excludeID)

	return s.nameBatchTaken, s.nameBatchErr
}

func (s *stubRepo) ExistsByBarcode(_ context.Context, barcode, excludeID string) (bool, error) {
	s.calls = append(s.calls, "barcode")
	s.gotBarcode = barcode
	s.gotExclude = append(s.gotExclude, excludeID)

	return s.barcodeTaken, s.barcodeErr
}

func (s *stubRepo) ExistsByLocation(_ context.Context, location int, excludeID string) (bool, error) {
	s.calls = append(s.calls, "location")
	s.gotLocation = location
	s.gotExclude = append(s.gotExclude, excludeID)

	return s.locationTaken, s.locationErr
}

func (s *stubRepo) ExistsByID(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "exists")

	return !s.idMissing, s.existsErr
}

func (s *stubRepo) LockByID(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "lock")

	return !s.lockMissing, s.lockErr
}

// LockQuantityByID is unused by this service's tests; it exists to satisfy
// medicine.Repository.
func (s *stubRepo) LockQuantityByID(_ context.Context, _ string) (int, bool, error) {
	return 0, !s.lockMissing, s.lockErr
}

func (s *stubRepo) ExistsInPurchaseOrder(_ context.Context, _ string) (bool, error) {
	s.calls = append(s.calls, "purchase_order")

	return s.inPurchase, s.purchaseErr
}

func (s *stubRepo) List(_ context.Context, filter medicine.ListFilter) ([]*domain.Medicine, int64, error) {
	s.calls = append(s.calls, "list")
	s.gotFilter = filter

	return s.listed, s.total, s.listErr
}

func (s *stubRepo) FindByBarcode(_ context.Context, barcode string) (*domain.Medicine, error) {
	s.calls = append(s.calls, "find")
	s.gotFindBarcode = barcode

	return s.found, s.findErr
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

func (s *stubRepo) Update(_ context.Context, m *domain.Medicine) error {
	s.calls = append(s.calls, "update")
	s.gotUpdate = m

	return s.updateErr
}

func (s *stubRepo) Delete(_ context.Context, id string) error {
	s.calls = append(s.calls, "delete")
	s.gotDeleteID = id

	return s.deleteErr
}

func (s *stubRepo) Transaction(_ context.Context, fn func(tx medicine.Repository) error) error {
	if s.txErr != nil {
		return s.txErr
	}

	return fn(s)
}

func (s *stubRepo) called(name string) bool {
	return slices.Contains(s.calls, name)
}

func input() CreateInput {
	batch := "B-001"
	location := 7

	return CreateInput{
		Name:           "Paracetamol",
		Barcode:        "8901234567890",
		BatchNumber:    &batch,
		ExpirationDate: time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		Quantity:       120,
		Location:       &location,
		CreatedBy:      "user-1",
	}
}

func updateInput() UpdateInput {
	batch := "B-002"
	location := 3

	return UpdateInput{
		Name:           "Paracetamol 500mg",
		Barcode:        "8901234567891",
		BatchNumber:    &batch,
		ExpirationDate: time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC),
		Location:       &location,
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
	want := []string{"name_batch", "barcode", "location", "create"}
	if !slices.Equal(repo.calls, want) {
		t.Fatalf("calls: got %v, want %v", repo.calls, want)
	}
	if repo.gotName != "Paracetamol" || repo.gotBarcode != "8901234567890" || repo.gotLocation != 7 {
		t.Errorf("checks got name %q barcode %q location %d", repo.gotName, repo.gotBarcode, repo.gotLocation)
	}
	if !slices.Equal(repo.gotExclude, []string{"", "", ""}) {
		t.Errorf("create must not exclude any medicine, got %q", repo.gotExclude)
	}
	c := repo.gotCreate
	if c.Name != "Paracetamol" || c.Barcode != "8901234567890" || c.Quantity != 120 {
		t.Errorf("created medicine: %+v", c)
	}
	if c.BatchNumber == nil || *c.BatchNumber != "B-001" {
		t.Errorf("batch number: got %v", c.BatchNumber)
	}
	if c.Location == nil || *c.Location != 7 {
		t.Errorf("location: got %v", c.Location)
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

func TestCreate_NilLocationSkipsTheCheck(t *testing.T) {
	repo := &stubRepo{locationTaken: true}
	svc := NewService(repo, zap.NewNop())
	in := input()
	in.Location = nil

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.called("location") {
		t.Error("an unplaced medicine must not check the location")
	}
	if repo.gotCreate.Location != nil {
		t.Errorf("location on the created medicine: got %v, want nil", *repo.gotCreate.Location)
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
		{"location taken", &stubRepo{locationTaken: true}, apperror.ErrLocationTaken, 3},
		{"both taken reports name and batch", &stubRepo{nameBatchTaken: true, barcodeTaken: true}, apperror.ErrNameBatchExists, 1},
		{"barcode and location taken reports barcode", &stubRepo{barcodeTaken: true, locationTaken: true}, apperror.ErrBarcodeExists, 2},
		{"insert conflict on barcode", &stubRepo{createErr: apperror.ErrBarcodeExists}, apperror.ErrBarcodeExists, 4},
		{"insert conflict on name and batch", &stubRepo{createErr: apperror.ErrNameBatchExists}, apperror.ErrNameBatchExists, 4},
		{"insert conflict on location", &stubRepo{createErr: apperror.ErrLocationTaken}, apperror.ErrLocationTaken, 4},
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
		"location check":       {locationErr: boom},
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

func TestList(t *testing.T) {
	first := &domain.Medicine{ID: "med-1"}
	repo := &stubRepo{listed: []*domain.Medicine{first}, total: 57}
	svc := NewService(repo, zap.NewNop())
	filter := ListFilter{Limit: 10, Offset: 20, Name: "para", Barcode: "890"}

	page, err := svc.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotFilter != filter {
		t.Errorf("filter: got %+v, want %+v passed through", repo.gotFilter, filter)
	}
	if page.Total != 57 || len(page.Medicines) != 1 || page.Medicines[0] != first {
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
		t.Errorf("error should wrap the repository error: %v", err)
	}
}

func TestGetByBarcode(t *testing.T) {
	want := &domain.Medicine{ID: "med-1", Barcode: "8901234567890"}
	repo := &stubRepo{found: want}
	svc := NewService(repo, zap.NewNop())

	got, err := svc.GetByBarcode(context.Background(), "8901234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != want {
		t.Errorf("medicine: got %+v, want %+v", got, want)
	}
	if repo.gotFindBarcode != "8901234567890" {
		t.Errorf("barcode: got %q", repo.gotFindBarcode)
	}
}

func TestGetByBarcode_Errors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("not found passes through", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: apperror.ErrNotFound}, zap.NewNop())

		got, err := svc.GetByBarcode(context.Background(), "1")

		if got != nil || err != apperror.ErrNotFound {
			t.Errorf("got %+v, %v; want nil and ErrNotFound unchanged", got, err)
		}
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: boom}, zap.NewNop())

		_, err := svc.GetByBarcode(context.Background(), "1")

		if !errors.Is(err, boom) || errors.Is(err, apperror.ErrNotFound) {
			t.Errorf("error should wrap the repository error only: %v", err)
		}
	})
}

func TestUpdate_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if err := svc.Update(context.Background(), "med-1", updateInput()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"exists", "name_batch", "barcode", "location", "update"}
	if !slices.Equal(repo.calls, want) {
		t.Fatalf("calls: got %v, want %v", repo.calls, want)
	}
	if !slices.Equal(repo.gotExclude, []string{"med-1", "med-1", "med-1"}) {
		t.Errorf("every check must exclude the medicine itself, got %q", repo.gotExclude)
	}

	u := repo.gotUpdate
	in := updateInput()
	if u.ID != "med-1" || u.Name != in.Name || u.Barcode != in.Barcode {
		t.Errorf("updated medicine: %+v", u)
	}
	if u.BatchNumber == nil || *u.BatchNumber != "B-002" {
		t.Errorf("batch number: got %v", u.BatchNumber)
	}
	if u.Location == nil || *u.Location != 3 || repo.gotLocation != 3 {
		t.Errorf("location: got %v on the update, %d on the check", u.Location, repo.gotLocation)
	}
	if !u.ExpirationDate.Equal(in.ExpirationDate) {
		t.Errorf("expiration date: got %v", u.ExpirationDate)
	}
	if u.Quantity != 0 {
		t.Errorf("quantity must not be part of an update, got %d", u.Quantity)
	}
}

func TestUpdate_NilBatchIsPassedThrough(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())
	in := updateInput()
	in.BatchNumber = nil

	if err := svc.Update(context.Background(), "med-1", in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotBatch != nil || repo.gotUpdate.BatchNumber != nil {
		t.Error("a cleared batch number must reach the check and the update as nil")
	}
}

func TestUpdate_NilLocationClearsItWithoutACheck(t *testing.T) {
	repo := &stubRepo{locationTaken: true}
	svc := NewService(repo, zap.NewNop())
	in := updateInput()
	in.Location = nil

	if err := svc.Update(context.Background(), "med-1", in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.called("location") {
		t.Error("clearing the location must not check it")
	}
	if repo.gotUpdate.Location != nil {
		t.Errorf("location on the update: got %v, want nil", *repo.gotUpdate.Location)
	}
}

func TestUpdate_UnknownIDStopsBeforeTheChecks(t *testing.T) {
	repo := &stubRepo{idMissing: true, nameBatchTaken: true, barcodeTaken: true}
	svc := NewService(repo, zap.NewNop())

	err := svc.Update(context.Background(), "med-1", updateInput())

	if err != apperror.ErrNotFound {
		t.Fatalf("error: got %v, want ErrNotFound unchanged", err)
	}
	if !slices.Equal(repo.calls, []string{"exists"}) {
		t.Errorf("calls: got %v, want only the existence check", repo.calls)
	}
}

func TestUpdate_Conflicts(t *testing.T) {
	tests := []struct {
		name      string
		repo      *stubRepo
		want      error
		wantCalls []string
	}{
		{"name and batch taken", &stubRepo{nameBatchTaken: true}, apperror.ErrNameBatchExists, []string{"exists", "name_batch"}},
		{"barcode taken", &stubRepo{barcodeTaken: true}, apperror.ErrBarcodeExists, []string{"exists", "name_batch", "barcode"}},
		{"location taken", &stubRepo{locationTaken: true}, apperror.ErrLocationTaken, []string{"exists", "name_batch", "barcode", "location"}},
		{"both taken reports name and batch", &stubRepo{nameBatchTaken: true, barcodeTaken: true}, apperror.ErrNameBatchExists, []string{"exists", "name_batch"}},
		{"update conflict on barcode", &stubRepo{updateErr: apperror.ErrBarcodeExists}, apperror.ErrBarcodeExists, []string{"exists", "name_batch", "barcode", "location", "update"}},
		{"update conflict on name and batch", &stubRepo{updateErr: apperror.ErrNameBatchExists}, apperror.ErrNameBatchExists, []string{"exists", "name_batch", "barcode", "location", "update"}},
		{"update conflict on location", &stubRepo{updateErr: apperror.ErrLocationTaken}, apperror.ErrLocationTaken, []string{"exists", "name_batch", "barcode", "location", "update"}},
		{"deleted in between", &stubRepo{updateErr: apperror.ErrNotFound}, apperror.ErrNotFound, []string{"exists", "name_batch", "barcode", "location", "update"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, zap.NewNop())

			err := svc.Update(context.Background(), "med-1", updateInput())

			if err != tt.want {
				t.Errorf("error: got %v, want %v unchanged", err, tt.want)
			}
			if !slices.Equal(tt.repo.calls, tt.wantCalls) {
				t.Errorf("calls: got %v, want %v", tt.repo.calls, tt.wantCalls)
			}
		})
	}
}

func TestUpdate_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := map[string]*stubRepo{
		"exists":               {existsErr: boom},
		"name and batch check": {nameBatchErr: boom},
		"barcode check":        {barcodeErr: boom},
		"location check":       {locationErr: boom},
		"update":               {updateErr: boom},
		"transaction":          {txErr: boom},
	}

	for name, repo := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewService(repo, zap.NewNop())

			err := svc.Update(context.Background(), "med-1", updateInput())

			if !errors.Is(err, boom) {
				t.Errorf("error should wrap the repository error: %v", err)
			}
			if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
				t.Error("a repository failure must not look like a conflict or not found")
			}
		})
	}
}

func TestDelete_Success(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo, zap.NewNop())

	if err := svc.Delete(context.Background(), "med-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"lock", "purchase_order", "delete"}
	if !slices.Equal(repo.calls, want) {
		t.Fatalf("calls: got %v, want %v", repo.calls, want)
	}
	if repo.gotDeleteID != "med-1" {
		t.Errorf("deleted id: got %q", repo.gotDeleteID)
	}
}

func TestDelete_UnknownIDStopsBeforeThePurchaseOrderCheck(t *testing.T) {
	repo := &stubRepo{lockMissing: true}
	svc := NewService(repo, zap.NewNop())

	err := svc.Delete(context.Background(), "med-1")

	if err != apperror.ErrNotFound {
		t.Fatalf("error: got %v, want ErrNotFound unchanged", err)
	}
	if repo.called("purchase_order") || repo.called("delete") {
		t.Errorf("calls: got %v, want only the lock", repo.calls)
	}
}

func TestDelete_MedicineInPurchaseOrderIsNotDeleted(t *testing.T) {
	repo := &stubRepo{inPurchase: true}
	svc := NewService(repo, zap.NewNop())

	err := svc.Delete(context.Background(), "med-1")

	if err != apperror.ErrMedicineInPurchaseOrder {
		t.Fatalf("error: got %v, want ErrMedicineInPurchaseOrder unchanged", err)
	}
	if !errors.Is(err, apperror.ErrConflict) {
		t.Error("error should wrap ErrConflict")
	}
	if repo.called("delete") {
		t.Error("a medicine used in a purchase order must not be deleted")
	}
}

func TestDelete_DeletedInBetween(t *testing.T) {
	svc := NewService(&stubRepo{deleteErr: apperror.ErrNotFound}, zap.NewNop())

	if err := svc.Delete(context.Background(), "med-1"); err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
}

func TestDelete_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := map[string]*stubRepo{
		"lock":           {lockErr: boom},
		"purchase order": {purchaseErr: boom},
		"delete":         {deleteErr: boom},
		"transaction":    {txErr: boom},
	}

	for name, repo := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewService(repo, zap.NewNop())

			err := svc.Delete(context.Background(), "med-1")

			if !errors.Is(err, boom) {
				t.Errorf("error should wrap the repository error: %v", err)
			}
			if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrNotFound) {
				t.Error("a repository failure must not look like a conflict or not found")
			}
		})
	}
}
