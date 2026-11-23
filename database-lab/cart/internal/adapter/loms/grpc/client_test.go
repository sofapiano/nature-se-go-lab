package grpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	lomsgrpc "github.com/igoroutine-courses/microservices.ecommerce.cart/internal/adapter/loms/grpc"
	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/entity"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
	productpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
	stockspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
)

const bufSize = 1 << 20

type fakeServers struct {
	productpb.UnimplementedProductServiceServer
	stockspb.UnimplementedStocksServer
	lomspb.UnimplementedLomsServer

	getProductFn  func(context.Context, *productpb.GetProductRequest) (*productpb.GetProductResponse, error)
	getStockFn    func(context.Context, *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error)
	createOrderFn func(context.Context, *lomspb.CreateOrderRequest) (*lomspb.CreateOrderResponse, error)
}

func (f *fakeServers) GetProduct(ctx context.Context, r *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
	return f.getProductFn(ctx, r)
}
func (f *fakeServers) GetStock(ctx context.Context, r *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error) {
	return f.getStockFn(ctx, r)
}
func (f *fakeServers) CreateOrder(ctx context.Context, r *lomspb.CreateOrderRequest) (*lomspb.CreateOrderResponse, error) {
	return f.createOrderFn(ctx, r)
}

func startServer(t *testing.T, fake *fakeServers) *googlegrpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := googlegrpc.NewServer()
	productpb.RegisterProductServiceServer(srv, fake)
	stockspb.RegisterStocksServer(srv, fake)
	lomspb.RegisterLomsServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := googlegrpc.NewClient(
		"passthrough://bufnet",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestClient_GetProduct_MapsFields — захватываем запрос и проверяем снаружи,
// чтобы не вызывать require внутри goroutine сервера (это сделало бы тест
// непредсказуемым при провале).
func TestClient_GetProduct_MapsFields(t *testing.T) {
	var gotReq *productpb.GetProductRequest
	fake := &fakeServers{
		getProductFn: func(_ context.Context, r *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
			gotReq = r
			return &productpb.GetProductResponse{Sku: "SKU-1", Name: "Widget", Price: 150, Active: true}, nil
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	got, err := c.GetProduct(context.Background(), "SKU-1")
	require.NoError(t, err)
	require.Equal(t, "SKU-1", gotReq.GetSku())
	require.Equal(t, entity.Product{SKU: "SKU-1", Name: "Widget", Price: 150, Active: true}, got)
}

func TestClient_GetProduct_NotFound(t *testing.T) {
	fake := &fakeServers{
		getProductFn: func(context.Context, *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
			return nil, status.Error(codes.NotFound, "no such sku")
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	_, err := c.GetProduct(context.Background(), "SKU-X")
	require.ErrorIs(t, err, entity.ErrNotFound)
}

// TestClient_GetProduct_NonNotFoundError — покрывает ветку fmt.Errorf в
// адаптере, которая не должна мапиться в ErrNotFound.
func TestClient_GetProduct_NonNotFoundError(t *testing.T) {
	fake := &fakeServers{
		getProductFn: func(context.Context, *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
			return nil, status.Error(codes.Internal, "boom")
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	_, err := c.GetProduct(context.Background(), "SKU-1")
	require.Error(t, err)
	require.NotErrorIs(t, err, entity.ErrNotFound)
	// status.Code проходит через fmt.Errorf-обёртку (grpc-go использует
	// errors.As внутри status.FromError), поэтому исходный код сохраняется.
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestClient_GetStock(t *testing.T) {
	fake := &fakeServers{
		getStockFn: func(context.Context, *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error) {
			return &stockspb.GetStockResponse{Count: 42}, nil
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	got, err := c.GetStock(context.Background(), "SKU-1")
	require.NoError(t, err)
	require.Equal(t, uint64(42), got)
}

func TestClient_GetStock_Error(t *testing.T) {
	fake := &fakeServers{
		getStockFn: func(context.Context, *stockspb.GetStockRequest) (*stockspb.GetStockResponse, error) {
			return nil, status.Error(codes.Unavailable, "loms down")
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	_, err := c.GetStock(context.Background(), "SKU-1")
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestClient_CreateOrder_PropagatesRequest(t *testing.T) {
	var captured *lomspb.CreateOrderRequest
	fake := &fakeServers{
		createOrderFn: func(_ context.Context, r *lomspb.CreateOrderRequest) (*lomspb.CreateOrderResponse, error) {
			captured = r
			return &lomspb.CreateOrderResponse{OrderId: "order-1"}, nil
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	items := []entity.Item{{SKU: "SKU-1", Count: 2}, {SKU: "SKU-2", Count: 1}}
	orderID, err := c.CreateOrder(context.Background(), "user-1", "req-1", items)
	require.NoError(t, err)
	require.Equal(t, "order-1", orderID)

	require.Equal(t, "user-1", captured.GetUserId())
	require.Equal(t, "req-1", captured.GetRequestId())
	require.Len(t, captured.GetItems(), 2)
	require.Equal(t, "SKU-1", captured.GetItems()[0].GetSku())
	require.Equal(t, uint32(2), captured.GetItems()[0].GetCount())
}

func TestClient_CreateOrder_PropagatesError(t *testing.T) {
	fake := &fakeServers{
		createOrderFn: func(context.Context, *lomspb.CreateOrderRequest) (*lomspb.CreateOrderResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
		},
	}
	c := lomsgrpc.New(startServer(t, fake))

	_, err := c.CreateOrder(context.Background(), "u", "r", []entity.Item{{SKU: "SKU-1", Count: 1}})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
