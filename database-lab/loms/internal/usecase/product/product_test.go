package product_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	. "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/product"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/usecase/product/mocks"
)

func setup(t *testing.T) (*Service, *mocks.Mockrepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockrepository(ctrl)
	return New(repo), repo
}

// ---------- Create ----------

// TestCreate_CanonicalizesSKU — usecase нормализует SKU (trim + upper) и
// принудительно выставляет Active=true. Клиент не должен «создать» снятый
// с продажи товар.
func TestCreate_CanonicalizesSKU(t *testing.T) {
	svc, repo := setup(t)

	expected := entity.Product{SKU: "SKU-1", Name: "Widget", Price: 100, Active: true}
	repo.EXPECT().Create(gomock.Any(), expected).Return(nil)

	sku, err := svc.Create(context.Background(), entity.Product{
		SKU:    "  sku-1  ",
		Name:   "Widget",
		Price:  100,
		Active: false, // будет перезаписано на true
	})
	require.NoError(t, err)
	require.Equal(t, "SKU-1", sku)
}

// TestCreate_RejectsInvalidInput_DoesNotCallRepo — при невалидном входе
// репозиторий НЕ вызывается. Это существенное поведение: невалидные данные
// не должны «дотекать» до хранилища.
func TestCreate_RejectsInvalidInput_DoesNotCallRepo(t *testing.T) {
	cases := []struct {
		name    string
		product entity.Product
	}{
		{"empty SKU", entity.Product{SKU: "", Name: "W", Price: 100}},
		{"whitespace-only name", entity.Product{SKU: "SKU-1", Name: "   ", Price: 100}},
		{"zero price", entity.Product{SKU: "SKU-1", Name: "W", Price: 0}},
		{"price over max", entity.Product{SKU: "SKU-1", Name: "W", Price: entity.MaxProductPrice + 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := setup(t) // repo.EXPECT() не объявлен — вызов уронит тест

			_, err := svc.Create(context.Background(), tc.product)
			require.ErrorIs(t, err, entity.ErrInvalidArgument)
		})
	}
}

// TestCreate_PropagatesRepoConflict — ошибка хранилища (ErrConflict) не
// должна превращаться в успех или Internal: клиенту нужен именно 409.
func TestCreate_PropagatesRepoConflict(t *testing.T) {
	svc, repo := setup(t)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.ErrConflict)

	_, err := svc.Create(context.Background(), entity.Product{SKU: "SKU-1", Name: "W", Price: 100})
	require.ErrorIs(t, err, entity.ErrConflict)
}

// ---------- Get ----------

func TestGet_CanonicalizesSKU(t *testing.T) {
	svc, repo := setup(t)

	repo.EXPECT().Get(gomock.Any(), "SKU-1").Return(entity.Product{SKU: "SKU-1", Name: "W"}, nil)

	got, err := svc.Get(context.Background(), "  sku-1  ")
	require.NoError(t, err)
	require.Equal(t, "SKU-1", got.SKU)
}

// ---------- Update ----------

func TestUpdate_CanonicalizesSKU(t *testing.T) {
	svc, repo := setup(t)

	expected := entity.Product{SKU: "SKU-1", Name: "W", Price: 200, Active: false}
	repo.EXPECT().Update(gomock.Any(), expected).Return(nil)

	require.NoError(t, svc.Update(context.Background(), entity.Product{
		SKU: "sku-1", Name: "W", Price: 200, Active: false,
	}))
}

func TestUpdate_RejectsInvalidInput(t *testing.T) {
	svc, _ := setup(t)

	err := svc.Update(context.Background(), entity.Product{SKU: "SKU-1", Name: "", Price: 100})
	require.ErrorIs(t, err, entity.ErrInvalidArgument)
}

// ---------- List ----------

func TestList_RejectsZeroLimit(t *testing.T) {
	svc, _ := setup(t)

	_, err := svc.List(context.Background(), "", 0)
	require.ErrorIs(t, err, entity.ErrInvalidArgument)
}

// TestList_CanonicalizesCursor — курсор (afterSKU) канонизируется перед
// запросом к репозиторию. Иначе поиск по индексу «upper-only» не найдёт
// позицию «sku-1».
func TestList_CanonicalizesCursor(t *testing.T) {
	svc, repo := setup(t)

	// usecase запрашивает limit+1, чтобы узнать, есть ли следующая страница.
	repo.EXPECT().List(gomock.Any(), "SKU-1", 3).
		Return([]entity.Product{{SKU: "SKU-2"}}, nil)

	page, err := svc.List(context.Background(), "  sku-1  ", 2)
	require.NoError(t, err)
	require.Len(t, page.Products, 1)
	require.Empty(t, page.NextAfterSKU) // меньше, чем limit — страница последняя
}

// TestList_TrimsToLimitAndReturnsCursor — ключевое свойство пагинации:
// курсор указывает на последний элемент текущей (усечённой) страницы, а не
// на «лишний», иначе на границе страниц элемент продублируется или пропадёт.
func TestList_TrimsToLimitAndReturnsCursor(t *testing.T) {
	svc, repo := setup(t)

	repo.EXPECT().List(gomock.Any(), "", 3).
		Return([]entity.Product{
			{SKU: "SKU-1"},
			{SKU: "SKU-2"},
			{SKU: "SKU-3"}, // «лишний» — только чтобы понять, что есть продолжение
		}, nil)

	page, err := svc.List(context.Background(), "", 2)
	require.NoError(t, err)
	require.Len(t, page.Products, 2)
	require.Equal(t, "SKU-1", page.Products[0].SKU)
	require.Equal(t, "SKU-2", page.Products[1].SKU)
	require.Equal(t, "SKU-2", page.NextAfterSKU)
}

// TestList_ShortPageNoCursor — ровно limit элементов = конец списка,
// курсора нет (нечего возвращать).
func TestList_ShortPageNoCursor(t *testing.T) {
	svc, repo := setup(t)

	repo.EXPECT().List(gomock.Any(), "", 3).
		Return([]entity.Product{{SKU: "SKU-1"}, {SKU: "SKU-2"}}, nil)

	page, err := svc.List(context.Background(), "", 2)
	require.NoError(t, err)
	require.Len(t, page.Products, 2)
	require.Empty(t, page.NextAfterSKU)
}
