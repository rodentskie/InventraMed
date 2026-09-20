package purchaseorder

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"

	"apps/api/internal/domain"
	medicinerepo "apps/api/internal/repository/medicine"
	"apps/api/internal/repository/purchaseorder"
	supplierrepo "apps/api/internal/repository/supplier"
	"apps/api/pkg/apperror"
)

const (
	supplierID = "b1a2c3d4-6a1e-4c3e-9d0e-5f1d2a7c9e11"
	medicineA  = "0b8f3c62-6a1e-4c3e-9d0e-5f1d2a7c9e11"
	medicineB  = "5c7d9e21-3b5c-4e8a-a1f6-9c0b2d4e6f88"
	orderID    = "e4f1a7b2-6a1e-4c3e-9d0e-5f1d2a7c9e11"
)

// stubRepo is a purchaseorder.Repository that also hands out a medicine and a
// supplier repository. All three record into the same calls slice, so a test
// can assert the order things happen in.
type stubRepo struct {
	status          string
	lockMissing     bool
	supplierMissing bool
	missingMedicine string
	items           []*domain.PurchaseOrderItem
	listed          []*domain.PurchaseOrder
	total           int64
	found           *domain.PurchaseOrder

	txErr       error
	supplierErr error
	medicineErr error
	createErr   error
	lockErr     error
	itemsErr    error
	receiptErr  error
	statusErr   error
	listErr     error
	findErr     error
	gotActor    string
	gotCreate   *domain.PurchaseOrder
	gotReceipt  *domain.PurchaseOrderReceipt
	gotStatus   string
	gotStatusID string
	gotFilter   purchaseorder.ListFilter
	gotFindID   string
	calls       []string
}

func newStubRepo() *stubRepo {
	return &stubRepo{status: domain.PurchaseOrderStatusDraft}
}

func (s *stubRepo) called(name string) bool {
	return slices.Contains(s.calls, name)
}

func (s *stubRepo) Create(_ context.Context, po *domain.PurchaseOrder) (*domain.PurchaseOrder, error) {
	s.calls = append(s.calls, "create")
	s.gotCreate = po
	if s.createErr != nil {
		return nil, s.createErr
	}

	created := *po
	created.ID = orderID

	return &created, nil
}

func (s *stubRepo) List(_ context.Context, filter purchaseorder.ListFilter) ([]*domain.PurchaseOrder, int64, error) {
	s.gotFilter = filter

	return s.listed, s.total, s.listErr
}

func (s *stubRepo) FindByID(_ context.Context, id string) (*domain.PurchaseOrder, error) {
	s.gotFindID = id

	return s.found, s.findErr
}

func (s *stubRepo) LockStatusByID(_ context.Context, _ string) (string, bool, error) {
	s.calls = append(s.calls, "lock_status")

	return s.status, !s.lockMissing, s.lockErr
}

func (s *stubRepo) ListItems(_ context.Context, _ string) ([]*domain.PurchaseOrderItem, error) {
	s.calls = append(s.calls, "list_items")

	return s.items, s.itemsErr
}

func (s *stubRepo) CreateReceipt(_ context.Context, receipt *domain.PurchaseOrderReceipt) (*domain.PurchaseOrderReceipt, error) {
	s.calls = append(s.calls, "create_receipt")
	s.gotReceipt = receipt
	if s.receiptErr != nil {
		return nil, s.receiptErr
	}

	created := *receipt
	created.ID = "receipt-1"

	return &created, nil
}

func (s *stubRepo) UpdateStatus(_ context.Context, id, status string) error {
	s.calls = append(s.calls, "update_status")
	s.gotStatusID = id
	s.gotStatus = status

	return s.statusErr
}

func (s *stubRepo) Transaction(
	_ context.Context,
	actorID string,
	fn func(tx purchaseorder.Repository, medicines medicinerepo.Repository, suppliers supplierrepo.Repository) error,
) error {
	s.calls = append(s.calls, "tx")
	s.gotActor = actorID
	if s.txErr != nil {
		return s.txErr
	}

	return fn(s, stubMedicines{s: s}, stubSuppliers{s: s})
}

// stubMedicines only implements LockByID; calling any other method panics,
// which is what a test of this service should want.
type stubMedicines struct {
	medicinerepo.Repository
	s *stubRepo
}

func (m stubMedicines) LockByID(_ context.Context, id string) (bool, error) {
	m.s.calls = append(m.s.calls, "lock_medicine:"+id)

	return id != m.s.missingMedicine, m.s.medicineErr
}

type stubSuppliers struct {
	supplierrepo.Repository
	s *stubRepo
}

func (m stubSuppliers) LockByID(_ context.Context, _ string) (bool, error) {
	m.s.calls = append(m.s.calls, "lock_supplier")

	return !m.s.supplierMissing, m.s.supplierErr
}

func str(value string) *string {
	return &value
}

func createInput() CreateInput {
	expected := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	return CreateInput{
		SupplierID:   supplierID,
		OrderDate:    time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		ExpectedDate: &expected,
		Notes:        str("Quarterly restock"),
		Items: []ItemInput{
			{MedicineID: medicineA, QuantityOrdered: 200},
			{MedicineID: medicineB, QuantityOrdered: 50},
		},
		CreatedBy: "user-1",
	}
}

func TestCreate_Success(t *testing.T) {
	repo := newStubRepo()
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Create(context.Background(), createInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != orderID {
		t.Errorf("id: got %q, want the repository's row", got.ID)
	}
	want := []string{"tx", "lock_supplier", "lock_medicine:" + medicineA, "lock_medicine:" + medicineB, "create"}
	if !slices.Equal(repo.calls, want) {
		t.Fatalf("calls: got %v, want %v", repo.calls, want)
	}
	if repo.gotActor != "user-1" {
		t.Errorf("actor: got %q, want the creator", repo.gotActor)
	}

	c := repo.gotCreate
	if c.SupplierID != supplierID || c.Status != domain.PurchaseOrderStatusDraft || c.CreatedBy != "user-1" {
		t.Errorf("created order: %+v", c)
	}
	if !c.OrderDate.Equal(createInput().OrderDate) || c.ExpectedDate == nil || !c.ExpectedDate.Equal(*createInput().ExpectedDate) {
		t.Errorf("dates: order %v expected %v", c.OrderDate, c.ExpectedDate)
	}
	if c.Notes == nil || *c.Notes != "Quarterly restock" {
		t.Errorf("notes: got %v", c.Notes)
	}
	if len(c.Items) != 2 || c.Items[0].MedicineID != medicineA || c.Items[0].QuantityOrdered != 200 ||
		c.Items[1].MedicineID != medicineB || c.Items[1].QuantityOrdered != 50 {
		t.Errorf("items: got %+v", c.Items)
	}
}

func TestCreate_NilOptionalFieldsArePassedThrough(t *testing.T) {
	repo := newStubRepo()
	svc := NewService(repo, zap.NewNop())
	in := createInput()
	in.ExpectedDate = nil
	in.Notes = nil

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotCreate.ExpectedDate != nil || repo.gotCreate.Notes != nil {
		t.Errorf("optional fields: got %+v, want nil", repo.gotCreate)
	}
}

func TestCreate_LocksMedicinesInAscendingOrderOnce(t *testing.T) {
	repo := newStubRepo()
	svc := NewService(repo, zap.NewNop())
	in := createInput()
	in.Items = []ItemInput{
		{MedicineID: medicineB, QuantityOrdered: 1},
		{MedicineID: medicineA, QuantityOrdered: 2},
		{MedicineID: medicineB, QuantityOrdered: 3},
	}

	if _, err := svc.Create(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"tx", "lock_supplier", "lock_medicine:" + medicineA, "lock_medicine:" + medicineB, "create"}
	if !slices.Equal(repo.calls, want) {
		t.Errorf("calls: got %v, want %v", repo.calls, want)
	}
	if len(repo.gotCreate.Items) != 3 {
		t.Errorf("items: got %d, want every item kept in the order as sent", len(repo.gotCreate.Items))
	}
}

func TestCreate_SupplierNotFoundStopsBeforeMedicinesAndInsert(t *testing.T) {
	repo := newStubRepo()
	repo.supplierMissing = true
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Create(context.Background(), createInput())

	if got != nil {
		t.Errorf("expected no order, got %+v", got)
	}
	if err != apperror.ErrSupplierNotFound {
		t.Errorf("error: got %v, want ErrSupplierNotFound unchanged", err)
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Error("ErrSupplierNotFound should wrap ErrNotFound")
	}
	if want := []string{"tx", "lock_supplier"}; !slices.Equal(repo.calls, want) {
		t.Errorf("calls: got %v, want %v", repo.calls, want)
	}
}

func TestCreate_MedicineNotFoundStopsBeforeInsert(t *testing.T) {
	repo := newStubRepo()
	repo.missingMedicine = medicineB
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Create(context.Background(), createInput())

	if got != nil {
		t.Errorf("expected no order, got %+v", got)
	}
	if err != apperror.ErrMedicineNotFound {
		t.Errorf("error: got %v, want ErrMedicineNotFound unchanged", err)
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Error("ErrMedicineNotFound should wrap ErrNotFound")
	}
	if repo.called("create") {
		t.Error("nothing may be inserted when a medicine is missing")
	}
}

func TestCreate_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		repo *stubRepo
	}{
		{"transaction", &stubRepo{txErr: boom}},
		{"supplier lock", &stubRepo{supplierErr: boom}},
		{"medicine lock", &stubRepo{medicineErr: boom}},
		{"insert", &stubRepo{createErr: boom}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, zap.NewNop())

			got, err := svc.Create(context.Background(), createInput())

			if got != nil {
				t.Errorf("expected no order, got %+v", got)
			}
			if !errors.Is(err, boom) {
				t.Errorf("error: got %v, want it to wrap %v", err, boom)
			}
		})
	}
}

func TestList_PassesTheFilterThrough(t *testing.T) {
	listed := []*domain.PurchaseOrder{{ID: "po-1"}, {ID: "po-2"}}
	repo := &stubRepo{listed: listed, total: 12}
	svc := NewService(repo, zap.NewNop())
	filter := ListFilter{Limit: 10, Offset: 20}

	page, err := svc.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotFilter != filter {
		t.Errorf("filter: got %+v, want %+v", repo.gotFilter, filter)
	}
	if page.Total != 12 || !slices.Equal(page.PurchaseOrders, listed) {
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
		want := &domain.PurchaseOrder{ID: orderID}
		repo := &stubRepo{found: want}
		svc := NewService(repo, zap.NewNop())

		got, err := svc.GetByID(context.Background(), orderID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != want {
			t.Errorf("order: got %+v, want %+v", got, want)
		}
		if repo.gotFindID != orderID {
			t.Errorf("id: got %q", repo.gotFindID)
		}
	})

	t.Run("not found passes through unchanged", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: apperror.ErrNotFound}, zap.NewNop())

		got, err := svc.GetByID(context.Background(), orderID)

		if got != nil {
			t.Errorf("expected no order, got %+v", got)
		}
		if err != apperror.ErrNotFound {
			t.Errorf("error: got %v, want ErrNotFound unchanged", err)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		svc := NewService(&stubRepo{findErr: boom}, zap.NewNop())

		_, err := svc.GetByID(context.Background(), orderID)

		if !errors.Is(err, boom) {
			t.Errorf("error: got %v, want it to wrap %v", err, boom)
		}
	})
}

func orderItems() []*domain.PurchaseOrderItem {
	return []*domain.PurchaseOrderItem{
		{ID: "item-1", PurchaseOrderID: orderID, MedicineID: medicineA, QuantityOrdered: 200},
		{ID: "item-2", PurchaseOrderID: orderID, MedicineID: medicineB, QuantityOrdered: 50},
	}
}

func receiveInput() ReceiveInput {
	return ReceiveInput{PurchaseOrderID: orderID, Notes: str("Delivered by courier"), ReceivedBy: "user-2"}
}

func TestReceive_Success(t *testing.T) {
	for _, status := range []string{domain.PurchaseOrderStatusDraft, domain.PurchaseOrderStatusOrdered} {
		t.Run(status, func(t *testing.T) {
			repo := newStubRepo()
			repo.status = status
			repo.items = orderItems()
			svc := NewService(repo, zap.NewNop())

			got, err := svc.Receive(context.Background(), receiveInput())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ID != "receipt-1" {
				t.Errorf("receipt: got %+v, want the repository's row", got)
			}
			want := []string{"tx", "lock_status", "list_items", "create_receipt", "update_status"}
			if !slices.Equal(repo.calls, want) {
				t.Fatalf("calls: got %v, want %v", repo.calls, want)
			}
			if repo.gotActor != "user-2" {
				t.Errorf("actor: got %q, want the receiver", repo.gotActor)
			}
			if repo.gotStatusID != orderID || repo.gotStatus != domain.PurchaseOrderStatusReceived {
				t.Errorf("status update: got %q on %q", repo.gotStatus, repo.gotStatusID)
			}
		})
	}
}

func TestReceive_ReceivesEveryItemInFull(t *testing.T) {
	repo := newStubRepo()
	repo.items = orderItems()
	svc := NewService(repo, zap.NewNop())

	if _, err := svc.Receive(context.Background(), receiveInput()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := repo.gotReceipt
	if r.PurchaseOrderID != orderID || r.ReceivedBy != "user-2" {
		t.Errorf("receipt: got %+v", r)
	}
	if r.Notes == nil || *r.Notes != "Delivered by courier" {
		t.Errorf("notes: got %v", r.Notes)
	}
	if len(r.Items) != 2 {
		t.Fatalf("receipt items: got %d, want one per order item", len(r.Items))
	}
	want := []struct {
		itemID   string
		received int
	}{{"item-1", 200}, {"item-2", 50}}
	for i, w := range want {
		got := r.Items[i]
		if got.PurchaseOrderItemID != w.itemID || got.QuantityReceived != w.received ||
			got.QuantityDamaged != 0 || got.QuantityReturned != 0 {
			t.Errorf("receipt item %d: got %+v, want %s received %d, nothing damaged or returned", i, got, w.itemID, w.received)
		}
	}
}

func TestReceive_NilNotesArePassedThrough(t *testing.T) {
	repo := newStubRepo()
	repo.items = orderItems()
	svc := NewService(repo, zap.NewNop())
	in := receiveInput()
	in.Notes = nil

	if _, err := svc.Receive(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.gotReceipt.Notes != nil {
		t.Errorf("notes: got %v, want nil", *repo.gotReceipt.Notes)
	}
}

func TestReceive_NotFoundStopsAfterTheLock(t *testing.T) {
	repo := newStubRepo()
	repo.lockMissing = true
	svc := NewService(repo, zap.NewNop())

	got, err := svc.Receive(context.Background(), receiveInput())

	if got != nil {
		t.Errorf("expected no receipt, got %+v", got)
	}
	if err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
	if want := []string{"tx", "lock_status"}; !slices.Equal(repo.calls, want) {
		t.Errorf("calls: got %v, want %v", repo.calls, want)
	}
}

func TestReceive_OtherStatusesAreNotReceivable(t *testing.T) {
	for _, status := range []string{
		domain.PurchaseOrderStatusReceived,
		domain.PurchaseOrderStatusCancelled,
		domain.PurchaseOrderStatusPartiallyReceived,
	} {
		t.Run(status, func(t *testing.T) {
			repo := newStubRepo()
			repo.status = status
			repo.items = orderItems()
			svc := NewService(repo, zap.NewNop())

			got, err := svc.Receive(context.Background(), receiveInput())

			if got != nil {
				t.Errorf("expected no receipt, got %+v", got)
			}
			if err != apperror.ErrPurchaseOrderNotReceivable {
				t.Errorf("error: got %v, want ErrPurchaseOrderNotReceivable unchanged", err)
			}
			if !errors.Is(err, apperror.ErrConflict) {
				t.Error("ErrPurchaseOrderNotReceivable should wrap ErrConflict")
			}
			if repo.called("create_receipt") || repo.called("update_status") {
				t.Errorf("calls: got %v, want nothing written", repo.calls)
			}
		})
	}
}

func TestReceive_RepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		repo *stubRepo
	}{
		{"transaction", &stubRepo{status: domain.PurchaseOrderStatusDraft, txErr: boom}},
		{"lock", &stubRepo{status: domain.PurchaseOrderStatusDraft, lockErr: boom}},
		{"list items", &stubRepo{status: domain.PurchaseOrderStatusDraft, itemsErr: boom}},
		{"create receipt", &stubRepo{status: domain.PurchaseOrderStatusDraft, receiptErr: boom, items: orderItems()}},
		{"update status", &stubRepo{status: domain.PurchaseOrderStatusDraft, statusErr: boom, items: orderItems()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(tt.repo, zap.NewNop())

			got, err := svc.Receive(context.Background(), receiveInput())

			if got != nil {
				t.Errorf("expected no receipt, got %+v", got)
			}
			if !errors.Is(err, boom) {
				t.Errorf("error: got %v, want it to wrap %v", err, boom)
			}
		})
	}
}

func TestReceive_NotFoundFromStatusUpdatePassesThrough(t *testing.T) {
	repo := newStubRepo()
	repo.items = orderItems()
	repo.statusErr = apperror.ErrNotFound
	svc := NewService(repo, zap.NewNop())

	_, err := svc.Receive(context.Background(), receiveInput())

	if err != apperror.ErrNotFound {
		t.Errorf("error: got %v, want ErrNotFound unchanged", err)
	}
}
