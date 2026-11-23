package stocks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	. "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/stocks"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/stocks/mocks"
)

type deps struct {
	svc      *Service
	stocks   *mocks.MockstocksRepository
	products *mocks.MockproductsRepository
}

func setup(t *testing.T) deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		stocks:   mocks.NewMockstocksRepository(ctrl),
		products: mocks.NewMockproductsRepository(ctrl),
	}
	d.svc = New(d.stocks, d.products)
	return d
}

const validOpID = "01900000-0000-7000-8000-000000000001"

// ---------- Get ----------

func TestGet_CanonicalizesSKU(t *testing.T) {
	d := setup(t)

	d.stocks.EXPECT().Get(gomock.Any(), "SKU-1").Return(uint64(7), nil)

	got, err := d.svc.Get(context.Background(), "  sku-1  ")
	require.NoError(t, err)
	require.Equal(t, uint64(7), got)
}

// ---------- Set ----------

func TestSet_RejectsExceedingMax(t *testing.T) {
	d := setup(t)

	err := d.svc.Set(context.Background(), "SKU-1", entity.MaxStock+1)
	require.ErrorIs(t, err, entity.ErrInvalidArgument)
}

// TestSet_UnknownProduct — существование проверяется в каталоге до вызова
// репозитория остатков. Иначе Set на неизвестный SKU создал бы «фантомный»
// остаток.
func TestSet_UnknownProduct(t *testing.T) {
	d := setup(t)

	d.products.EXPECT().Get(gomock.Any(), "SKU-X").Return(entity.Product{}, entity.ErrNotFound)
	// stocks.Set НЕ ожидается

	err := d.svc.Set(context.Background(), "SKU-X", 5)
	require.ErrorIs(t, err, entity.ErrNotFound)
}

func TestSet_CanonicalizesAndDelegates(t *testing.T) {
	d := setup(t)

	d.products.EXPECT().Get(gomock.Any(), "SKU-1").Return(entity.Product{SKU: "SKU-1"}, nil)
	d.stocks.EXPECT().Set(gomock.Any(), "SKU-1", uint64(5)).Return(nil)

	require.NoError(t, d.svc.Set(context.Background(), "  sku-1  ", 5))
}

// ---------- Receive ----------

// TestReceive_RejectsInvalidInput — все три ветки валидации, одинаково
// недоступные до вызова зависимостей.
func TestReceive_RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		receipt entity.StockReceipt
	}{
		{"empty operation_id", entity.StockReceipt{SKU: "SKU-1", Count: 1, OperationID: ""}},
		{"zero count", entity.StockReceipt{SKU: "SKU-1", Count: 0, OperationID: validOpID}},
		{"count over max", entity.StockReceipt{SKU: "SKU-1", Count: entity.MaxStock + 1, OperationID: validOpID}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setup(t) // ни products.Get, ни stocks.Receive не ожидаются

			err := d.svc.Receive(context.Background(), tc.receipt)
			require.ErrorIs(t, err, entity.ErrInvalidArgument)
		})
	}
}

func TestReceive_PropagatesUnknownProduct(t *testing.T) {
	d := setup(t)

	d.products.EXPECT().Get(gomock.Any(), "SKU-X").Return(entity.Product{}, entity.ErrNotFound)

	err := d.svc.Receive(context.Background(), entity.StockReceipt{
		SKU: "SKU-X", Count: 1, OperationID: validOpID,
	})
	require.ErrorIs(t, err, entity.ErrNotFound)
}

// TestReceive_CanonicalizesFieldsAndDelegates — usecase нормализует SKU
// (trim+upper) и operation_id (lower) до вызова репозитория. Иначе повтор с
// UPPER-ключом был бы воспринят как новая операция.
func TestReceive_CanonicalizesFieldsAndDelegates(t *testing.T) {
	d := setup(t)

	expected := entity.StockReceipt{
		SKU:         "SKU-1",
		Count:       5,
		OperationID: "01900000-0000-7000-8000-aaaaaaaaaaaa",
	}
	d.products.EXPECT().Get(gomock.Any(), "SKU-1").Return(entity.Product{SKU: "SKU-1"}, nil)
	d.stocks.EXPECT().Receive(gomock.Any(), expected).Return(nil)

	err := d.svc.Receive(context.Background(), entity.StockReceipt{
		SKU:         "  sku-1  ",
		Count:       5,
		OperationID: "01900000-0000-7000-8000-AAAAAAAAAAAA",
	})
	require.NoError(t, err)
}

// TestReceive_PropagatesRepoConflict — ErrConflict от репозитория
// (ключ использован с другими данными) не должен теряться.
func TestReceive_PropagatesRepoConflict(t *testing.T) {
	d := setup(t)

	d.products.EXPECT().Get(gomock.Any(), "SKU-1").Return(entity.Product{SKU: "SKU-1"}, nil)
	d.stocks.EXPECT().Receive(gomock.Any(), gomock.Any()).Return(entity.ErrConflict)

	err := d.svc.Receive(context.Background(), entity.StockReceipt{
		SKU: "SKU-1", Count: 1, OperationID: validOpID,
	})
	require.ErrorIs(t, err, entity.ErrConflict)
}
