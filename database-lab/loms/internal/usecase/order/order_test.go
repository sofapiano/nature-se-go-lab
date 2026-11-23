package order

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/order/mocks"
)

const (
	userID    = "01900000-0000-7000-8000-000000000001"
	requestID = "01900000-0000-7000-8000-000000000002"
	orderID   = "01900000-0000-7000-8000-000000000003"
)

// noopTransactor — заглушка для юнит-тестов, просто выполняет функцию без реальной БД-транзакции.
type noopTransactor struct{}

func (n *noopTransactor) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type deps struct {
	svc        *Service
	orders     *mocks.MockorderRepository
	products   *mocks.MockproductRepository
	stocks     *mocks.MockstocksRepository
	billing    *mocks.MockbillingClient
	notifier   *mocks.MocknotificationClient
	outbox     *mocks.MockoutboxRepository
	transactor *noopTransactor
}

func setup(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		orders:     mocks.NewMockorderRepository(ctrl),
		products:   mocks.NewMockproductRepository(ctrl),
		stocks:     mocks.NewMockstocksRepository(ctrl),
		billing:    mocks.NewMockbillingClient(ctrl),
		notifier:   mocks.NewMocknotificationClient(ctrl),
		outbox:     mocks.NewMockoutboxRepository(ctrl),
		transactor: &noopTransactor{},
	}
	d.svc = New(d.orders, d.products, d.stocks, d.billing, d.notifier, d.transactor, d.outbox)
	return d
}

// captureUpdate — подписывается на Update и сохраняет последний Order.
func captureUpdate(d deps) *entity.Order {
	var captured entity.Order
	d.orders.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, o entity.Order) { captured = o }).
		AnyTimes()
	return &captured
}

// ---------- CreateOrder ----------

func TestCreateOrder_Success(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(entity.OrderRequest{}, false)
	d.products.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "Widget", Price: 100, Active: true}, nil)
	d.stocks.EXPECT().Reserve(gomock.Any(), []entity.Item{{SKU: "SKU-1", Count: 2, Name: "Widget", Price: 100}}).Return(nil)

	var saved entity.Order
	d.orders.EXPECT().Save(gomock.Any(), gomock.Any(), requestID, gomock.Any()).
		DoAndReturn(func(_ context.Context, o entity.Order, _ string, _ []entity.Item) { saved = o })
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	id, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "sku-1", Count: 2}})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.Equal(t, entity.StatusAwaitingPayment, saved.Status)
	require.Equal(t, uint32(200), saved.TotalAmountMinor)
	require.Len(t, saved.Items, 1)
	require.Equal(t, "Widget", saved.Items[0].Name)
	require.Equal(t, uint32(100), saved.Items[0].Price)
	require.Len(t, saved.History, 1)
}

func TestCreateOrder_IdempotentReplay_ReturnsSameOrder_NoReservation(t *testing.T) {
	d := setup(t)

	existing := entity.OrderRequest{
		UserID:    userID,
		Items:     []entity.Item{{SKU: "SKU-1", Count: 2}},
		RequestID: requestID,
		OrderID:   orderID,
	}
	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(existing, true)

	id, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "SKU-1", Count: 2}})
	require.NoError(t, err)
	require.Equal(t, orderID, id)
}

func TestCreateOrder_ConflictOnDifferentPayload(t *testing.T) {
	d := setup(t)

	existing := entity.OrderRequest{
		UserID:    userID,
		Items:     []entity.Item{{SKU: "SKU-1", Count: 2}},
		RequestID: requestID,
		OrderID:   orderID,
	}
	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(existing, true)

	_, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "SKU-1", Count: 5}})
	require.ErrorIs(t, err, entity.ErrConflict)
}

func TestCreateOrder_RejectsEmptyAndDuplicateSKUs(t *testing.T) {
	cases := []struct {
		name  string
		items []entity.Item
	}{
		{"empty sku", []entity.Item{{SKU: "", Count: 1}}},
		{"zero count", []entity.Item{{SKU: "SKU-1", Count: 0}}},
		{"duplicate after normalization", []entity.Item{
			{SKU: "SKU-1", Count: 1},
			{SKU: "sku-1", Count: 2},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setup(t)
			_, err := d.svc.CreateOrder(context.Background(), userID, requestID, tc.items)
			require.ErrorIs(t, err, entity.ErrInvalidArgument)
		})
	}
}

func TestCreateOrder_InactiveProduct_NoReservation(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(entity.OrderRequest{}, false)
	d.products.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Active: false}, nil)

	_, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "SKU-1", Count: 1}})
	require.ErrorIs(t, err, entity.ErrState)
}

func TestCreateOrder_ReserveFailure_NoSave(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(entity.OrderRequest{}, false)
	d.products.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "W", Price: 100, Active: true}, nil)
	d.stocks.EXPECT().Reserve(gomock.Any(), gomock.Any()).Return(entity.ErrStock)

	_, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "SKU-1", Count: 1}})
	require.ErrorIs(t, err, entity.ErrStock)
}

func TestCreateOrder_OverflowReturnsErrState(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(entity.OrderRequest{}, false)

	items := make([]entity.Item, 5)
	for i := range items {
		sku := "SKU-" + string(rune('A'+i))
		items[i] = entity.Item{SKU: sku, Count: 1000}
		d.products.EXPECT().Get(gomock.Any(), sku).
			Return(entity.Product{SKU: sku, Name: "W", Price: 1_000_000, Active: true}, nil)
	}

	_, err := d.svc.CreateOrder(context.Background(), userID, requestID, items)
	require.ErrorIs(t, err, entity.ErrState)
}

func TestCreateOrder_OutboxSaveSucceeds(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).Return(entity.OrderRequest{}, false)
	d.products.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "W", Price: 100, Active: true}, nil)
	d.stocks.EXPECT().Reserve(gomock.Any(), gomock.Any()).Return(nil)
	d.orders.EXPECT().Save(gomock.Any(), gomock.Any(), requestID, gomock.Any())
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	id, err := d.svc.CreateOrder(context.Background(), userID, requestID,
		[]entity.Item{{SKU: "SKU-1", Count: 1}})
	require.NoError(t, err)
	require.NotEmpty(t, id)
}

func TestCreateOrder_ConcurrentSameKey_OneOrder(t *testing.T) {
	d := setup(t)

	var mu sync.Mutex
	calls := 0
	d.orders.EXPECT().GetRequest(gomock.Any(), requestID).DoAndReturn(
		func(context.Context, string) (entity.OrderRequest, bool) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return entity.OrderRequest{}, false
			}
			return entity.OrderRequest{
				UserID:    userID,
				RequestID: requestID,
				OrderID:   orderID,
				Items:     []entity.Item{{SKU: "SKU-1", Count: 1}},
			}, true
		}).Times(8)

	d.products.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "W", Price: 100, Active: true}, nil).Times(1)
	d.stocks.EXPECT().Reserve(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	d.orders.EXPECT().Save(gomock.Any(), gomock.Any(), requestID, gomock.Any()).Times(1)
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	const n = 8
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			ids[i], errs[i] = d.svc.CreateOrder(context.Background(), userID, requestID,
				[]entity.Item{{SKU: "SKU-1", Count: 1}})
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "goroutine %d", i)
		require.NotEmpty(t, ids[i])
	}
}

// ---------- GetOrder / ListOrders ----------

func TestGetOrder_CanonicalizesID(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), strings.ToLower(orderID)).
		Return(entity.Order{OrderID: orderID}, nil)

	got, err := d.svc.GetOrder(context.Background(), strings.ToUpper(orderID))
	require.NoError(t, err)
	require.Equal(t, orderID, got.OrderID)
}

func TestListOrders_DefaultsAndPagination(t *testing.T) {
	cases := []struct {
		name       string
		inputLimit uint32
		expected   int
	}{
		{"zero → default 20", 0, 21},
		{"explicit", 5, 6},
		{"over max → clamp 100", 200, 101},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setup(t)
			d.orders.EXPECT().ListByUser(gomock.Any(), userID, "", tc.expected).
				Return(nil, nil)

			_, err := d.svc.ListOrders(context.Background(), userID, "", tc.inputLimit)
			require.NoError(t, err)
		})
	}
}

func TestListOrders_TrimsPageAndReturnsCursor(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().ListByUser(gomock.Any(), userID, "", 3).
		Return([]entity.Order{{OrderID: "a"}, {OrderID: "b"}, {OrderID: "c"}}, nil)

	page, err := d.svc.ListOrders(context.Background(), userID, "", 2)
	require.NoError(t, err)
	require.Len(t, page.Orders, 2)
	require.Equal(t, "b", page.NextAfterOrderID)
}

// ---------- PayOrder ----------

func TestPayOrder_UnknownOrder(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{}, entity.ErrNotFound)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestPayOrder_AlreadyPaid_NoNewCharge(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).
		Return(entity.Order{OrderID: orderID, Status: entity.StatusPaid}, nil)

	require.NoError(t, d.svc.PayOrder(context.Background(), orderID))
}

func TestPayOrder_CancelledReturnsErrState(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).
		Return(entity.Order{OrderID: orderID, Status: entity.StatusCancelled}, nil)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrState)
}

func TestPayOrder_Succeeded(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	var sentExternalID string
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusAwaitingPayment,
		TotalAmountMinor: 300,
		Currency:         "RUB",
	}, nil)
	d.orders.EXPECT().Get(gomock.Any(), orderID).DoAndReturn(func(_ context.Context, _ string) (entity.Order, error) {
		return entity.Order{
			OrderID:          orderID,
			UserID:           userID,
			Status:           entity.StatusPaymentPending,
			TotalAmountMinor: 300,
			Currency:         "RUB",
			Payments:         []entity.PaymentSummary{{ExternalID: sentExternalID, AmountMinor: 300, Currency: "RUB", Status: entity.PaymentStatusPending}},
			History:          []entity.Transition{{Status: entity.StatusPaymentPending}},
		}, nil
	})
	d.billing.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), userID, uint64(300), "RUB").
		DoAndReturn(func(_ context.Context, extID, _ string, _ uint64, _ string) (entity.PaymentSummary, error) {
			sentExternalID = extID
			return entity.PaymentSummary{
				PaymentID:   "pm-1",
				ExternalID:  extID,
				AmountMinor: 300,
				Currency:    "RUB",
				Status:      entity.PaymentStatusSucceeded,
			}, nil
		})
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	require.NoError(t, d.svc.PayOrder(context.Background(), orderID))
	require.NotEmpty(t, sentExternalID)
	require.Equal(t, entity.StatusPaid, updated.Status)
	require.Equal(t, "pm-1", updated.Payments[len(updated.Payments)-1].PaymentID)
	require.Equal(t, sentExternalID, updated.Payments[len(updated.Payments)-1].ExternalID)
	require.Len(t, updated.History, 2)
}

func TestPayOrder_FirstDecline_BackToAwaiting(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	var sentExternalID string
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusAwaitingPayment,
		TotalAmountMinor: 100,
		Currency:         "RUB",
	}, nil)
	d.orders.EXPECT().Get(gomock.Any(), orderID).DoAndReturn(func(_ context.Context, _ string) (entity.Order, error) {
		return entity.Order{
			OrderID:          orderID,
			UserID:           userID,
			Status:           entity.StatusPaymentPending,
			TotalAmountMinor: 100,
			Currency:         "RUB",
			Payments:         []entity.PaymentSummary{{ExternalID: sentExternalID, AmountMinor: 100, Currency: "RUB", Status: entity.PaymentStatusPending}},
			History:          []entity.Transition{{Status: entity.StatusPaymentPending}},
		}, nil
	})

	d.billing.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), userID, uint64(100), "RUB").
		DoAndReturn(func(_ context.Context, extID, _ string, _ uint64, _ string) (entity.PaymentSummary, error) {
			sentExternalID = extID
			return entity.PaymentSummary{
				PaymentID: "pm-1", ExternalID: extID,
				Status: entity.PaymentStatusDeclined,
			}, nil
		})
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrDeclined)
	require.Equal(t, entity.StatusAwaitingPayment, updated.Status)
}

func TestPayOrder_ThirdDecline_FailsAndReleases(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	seeded := entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusPaymentPending,
		TotalAmountMinor: 100,
		Currency:         "RUB",
		Items:            []entity.Item{{SKU: "SKU-1", Count: 2}},
		Payments: []entity.PaymentSummary{
			{ExternalID: "ext-1", Status: entity.PaymentStatusDeclined, PaymentID: "pm-1"},
			{ExternalID: "ext-2", Status: entity.PaymentStatusDeclined, PaymentID: "pm-2"},
			{ExternalID: "ext-3", Status: entity.PaymentStatusPending},
		},
	}
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(seeded, nil).Times(2)

	d.billing.EXPECT().GetPayment(gomock.Any(), "ext-3").
		Return(entity.PaymentSummary{
			PaymentID: "pm-3", ExternalID: "ext-3",
			Status: entity.PaymentStatusDeclined,
		}, nil)
	d.stocks.EXPECT().Release(gomock.Any(), seeded.Items).Return(nil)
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrDeclined)
	require.Equal(t, entity.StatusFailed, updated.Status)
}

func TestPayOrder_PaymentPending_GetPaymentSucceeds(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	seeded := entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusPaymentPending,
		TotalAmountMinor: 500,
		Currency:         "RUB",
		Payments:         []entity.PaymentSummary{{ExternalID: "ext-1", Status: entity.PaymentStatusPending}},
	}
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(seeded, nil).Times(2)

	d.billing.EXPECT().GetPayment(gomock.Any(), "ext-1").
		Return(entity.PaymentSummary{
			PaymentID: "pm-1", ExternalID: "ext-1",
			Status: entity.PaymentStatusSucceeded,
		}, nil)
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	require.NoError(t, d.svc.PayOrder(context.Background(), orderID))
	require.Equal(t, entity.StatusPaid, updated.Status)
}

func TestPayOrder_PaymentPending_GetPaymentNotFound_FallbackToCreate(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	seeded := entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusPaymentPending,
		TotalAmountMinor: 500,
		Currency:         "RUB",
		Payments:         []entity.PaymentSummary{{ExternalID: "ext-1", Status: entity.PaymentStatusPending}},
	}
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(seeded, nil).Times(2)

	d.billing.EXPECT().GetPayment(gomock.Any(), "ext-1").
		Return(entity.PaymentSummary{}, status.Error(codes.NotFound, "no payment"))
	d.billing.EXPECT().CreatePayment(gomock.Any(), "ext-1", userID, uint64(500), "RUB").
		Return(entity.PaymentSummary{
			PaymentID: "pm-1", ExternalID: "ext-1",
			Status: entity.PaymentStatusSucceeded,
		}, nil)
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	require.NoError(t, d.svc.PayOrder(context.Background(), orderID))
	require.Equal(t, entity.StatusPaid, updated.Status)
}

func TestPayOrder_BillingError_StaysPending(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID:          orderID,
		UserID:           userID,
		Status:           entity.StatusAwaitingPayment,
		TotalAmountMinor: 100,
		Currency:         "RUB",
	}, nil)

	networkErr := status.Error(codes.Unavailable, "billing down")
	d.billing.EXPECT().CreatePayment(gomock.Any(), gomock.Any(), userID, uint64(100), "RUB").
		Return(entity.PaymentSummary{}, networkErr)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.Error(t, err)
	require.Equal(t, entity.StatusPaymentPending, updated.Status)
	require.Len(t, updated.Payments, 1)
	require.Equal(t, entity.PaymentStatusPending, updated.Payments[0].Status)
	require.NotEmpty(t, updated.Payments[0].ExternalID)
}

func TestPayOrder_PaymentPendingWithoutRecord(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID,
		Status:  entity.StatusPaymentPending,
	}, nil)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.Equal(t, codes.Internal, status.Code(err))
}

// ---------- CancelOrder ----------

func TestCancelOrder_UnknownOrder(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{}, entity.ErrNotFound)

	err := d.svc.CancelOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestCancelOrder_NotAwaiting_ReturnsErrState(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID,
		Status:  entity.StatusPaid,
	}, nil)

	err := d.svc.CancelOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrState)
}

func TestCancelOrder_Success(t *testing.T) {
	d := setup(t)
	updated := captureUpdate(d)

	items := []entity.Item{{SKU: "SKU-1", Count: 2}}
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID,
		UserID:  userID,
		Status:  entity.StatusAwaitingPayment,
		Items:   items,
	}, nil)
	d.stocks.EXPECT().Release(gomock.Any(), items).Return(nil)
	d.outbox.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	require.NoError(t, d.svc.CancelOrder(context.Background(), orderID))
	require.Equal(t, entity.StatusCancelled, updated.Status)
}

func TestCancelOrder_ReleaseFailure_StateUnchanged(t *testing.T) {
	d := setup(t)

	items := []entity.Item{{SKU: "SKU-1", Count: 2}}
	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID,
		UserID:  userID,
		Status:  entity.StatusAwaitingPayment,
		Items:   items,
	}, nil)
	d.stocks.EXPECT().Release(gomock.Any(), items).Return(errors.New("stock down"))

	err := d.svc.CancelOrder(context.Background(), orderID)
	require.Error(t, err)
}

func TestPayOrder_AfterCancelled_ReturnsErrState(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID, Status: entity.StatusCancelled,
	}, nil)

	err := d.svc.PayOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrState)
}

func TestCancelOrder_AfterPaid_ReturnsErrState(t *testing.T) {
	d := setup(t)

	d.orders.EXPECT().Get(gomock.Any(), orderID).Return(entity.Order{
		OrderID: orderID, Status: entity.StatusPaid,
	}, nil)

	err := d.svc.CancelOrder(context.Background(), orderID)
	require.ErrorIs(t, err, entity.ErrState)
}