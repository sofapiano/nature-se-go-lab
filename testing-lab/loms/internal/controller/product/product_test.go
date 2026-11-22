package product_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	. "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/product"
	mocks "github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/product/mocks"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	productpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
)

func setup(t *testing.T) (*Controller, *mocks.MockproductService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockproductService(ctrl)
	return New(svc), svc
}

func TestCreateProduct_MapsRequestAndResponse(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().Create(gomock.Any(), entity.Product{SKU: "sku-1", Name: "W", Price: 100}).
		Return("SKU-1", nil)

	resp, err := c.CreateProduct(context.Background(), &productpb.CreateProductRequest{
		Sku: "sku-1", Name: "W", Price: 100,
	})
	require.NoError(t, err)
	require.Equal(t, "SKU-1", resp.GetSku())
}

func TestCreateProduct_ConflictMapsToAlreadyExists(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().Create(gomock.Any(), gomock.Any()).Return("", entity.ErrConflict)

	_, err := c.CreateProduct(context.Background(), &productpb.CreateProductRequest{Sku: "SKU-1"})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestGetProduct_Success(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().Get(gomock.Any(), "SKU-1").
		Return(entity.Product{SKU: "SKU-1", Name: "W", Price: 100, Active: true}, nil)

	resp, err := c.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: "SKU-1"})
	require.NoError(t, err)
	require.Equal(t, "SKU-1", resp.GetSku())
	require.Equal(t, "W", resp.GetName())
	require.Equal(t, uint32(100), resp.GetPrice())
	require.True(t, resp.GetActive())
}

func TestGetProduct_NotFound(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Get(gomock.Any(), "SKU-X").Return(entity.Product{}, entity.ErrNotFound)

	_, err := c.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: "SKU-X"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

// TestListProducts_PassesPageSizeAndMapsPage — важно, что limit=0 в запросе
// превращается в PageSize()-дефолт до вызова usecase. Usecase не должен
// видеть «сырой» ноль.
func TestListProducts_PassesPageSizeAndMapsPage(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().List(gomock.Any(), "", 50). // PageSize(0) = 50
							Return(entity.ProductPage{
			Products:     []entity.Product{{SKU: "SKU-1", Name: "W", Price: 100, Active: true}},
			NextAfterSKU: "SKU-1",
		}, nil)

	resp, err := c.ListProducts(context.Background(), &productpb.ListProductsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetProducts(), 1)
	require.Equal(t, "SKU-1", resp.GetNextAfterSku())
}

func TestListProducts_OverMaxClampsTo100(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().List(gomock.Any(), "", 100).Return(entity.ProductPage{}, nil)

	_, err := c.ListProducts(context.Background(), &productpb.ListProductsRequest{Limit: 500})
	require.NoError(t, err)
}

func TestUpdateProduct_Success(t *testing.T) {
	c, svc := setup(t)

	svc.EXPECT().Update(gomock.Any(), entity.Product{SKU: "SKU-1", Name: "W", Price: 200, Active: false}).
		Return(nil)

	_, err := c.UpdateProduct(context.Background(), &productpb.UpdateProductRequest{
		Sku: "SKU-1", Name: "W", Price: 200, Active: false,
	})
	require.NoError(t, err)
}

func TestUpdateProduct_PropagatesError(t *testing.T) {
	c, svc := setup(t)
	svc.EXPECT().Update(gomock.Any(), gomock.Any()).Return(entity.ErrNotFound)

	_, err := c.UpdateProduct(context.Background(), &productpb.UpdateProductRequest{Sku: "SKU-X"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

// guard: если кто-то поменяет маппинг entity -> pb и забудет поле,
// этот тест не поймает, но `toProto`-подобной хелперной функции тут нет.
// Просто держим `errors` в импортах задействованным через sentinel.
var _ = errors.New
