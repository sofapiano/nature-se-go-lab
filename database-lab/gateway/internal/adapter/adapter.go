package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/igoroutine-courses/microservices.ecommerce.gateway/internal/httpapi"
	cartpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/cart/api/cart/v1"
	lomspb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/loms/v1"
	product "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
	stocks "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/stocks/v1"
	notifierpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/notifier/api/v1"
)

type backend struct {
	cart     cartpb.CartClient
	loms     lomspb.LomsClient
	product  product.ProductServiceClient
	stocks   stocks.StocksClient
	notifier notifierpb.NotifierClient // добавлено
}

const invalidRequest = "invalid request"

type Backend interface {
	Call(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

var _ Backend = (*backend)(nil)

var (
	unmarshaler = protojson.UnmarshalOptions{DiscardUnknown: true}
	marshaler   = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
)

func New(cartAddress, lomsAddress, notifierAddress string) (*backend, func(), error) {
	cartConn, err := grpc.NewClient(cartAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to cart %q: %w", cartAddress, err)
	}
	lomsConn, err := grpc.NewClient(lomsAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = cartConn.Close()
		return nil, nil, fmt.Errorf("connect to loms %q: %w", lomsAddress, err)
	}
	notifierConn, err := grpc.NewClient(notifierAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = cartConn.Close()
		_ = lomsConn.Close()
		return nil, nil, fmt.Errorf("connect to notifier %q: %w", notifierAddress, err)
	}

	b := &backend{
		cart:     cartpb.NewCartClient(cartConn),
		loms:     lomspb.NewLomsClient(lomsConn),
		product:  product.NewProductServiceClient(lomsConn),
		stocks:   stocks.NewStocksClient(lomsConn),
		notifier: notifierpb.NewNotifierClient(notifierConn),
	}
	closeConnections := func() {
		_ = cartConn.Close()
		_ = lomsConn.Close()
		_ = notifierConn.Close()
	}
	return b, closeConnections, nil
}

func (b *backend) Call(ctx context.Context, path string, raw json.RawMessage) (json.RawMessage, error) {
	switch path {
	case "/v1/product/create":
		return unary(ctx, raw, &product.CreateProductRequest{}, b.product.CreateProduct)
	case "/v1/product/list":
		return unary(ctx, raw, &product.ListProductsRequest{}, b.product.ListProducts)
	case "/v1/product/info":
		return unary(ctx, raw, &product.GetProductRequest{}, b.product.GetProduct)
	case "/v1/product/update":
		return unary(ctx, raw, &product.UpdateProductRequest{}, b.product.UpdateProduct)
	case "/v1/stock/info":
		return unary(ctx, raw, &stocks.GetStockRequest{}, b.stocks.GetStock)
	case "/v1/stock/set":
		return unary(ctx, raw, &stocks.SetStockRequest{}, b.stocks.SetStock)
	case "/v1/stock/receive":
		return unary(ctx, raw, &stocks.ReceiveStockRequest{}, b.stocks.ReceiveStock)

	case "/v1/cart/item/add":
		return unary(ctx, raw, &cartpb.AddItemRequest{}, b.cart.AddItem)
	case "/v1/cart/item/set":
		return unary(ctx, raw, &cartpb.SetItemQuantityRequest{}, b.cart.SetItemQuantity)
	case "/v1/cart/item/delete":
		return unary(ctx, raw, &cartpb.DeleteItemRequest{}, b.cart.DeleteItem)
	case "/v1/cart/clear":
		return unary(ctx, raw, &cartpb.ClearCartRequest{}, b.cart.ClearCart)
	case "/v1/cart/list":
		return cartList(ctx, raw, b.cart.ListCart)
	case "/v1/cart/checkout":
		return unary(ctx, raw, &cartpb.CheckoutCartRequest{}, b.cart.CheckoutCart)

	case "/v1/order/create":
		return unary(ctx, raw, &lomspb.CreateOrderRequest{}, b.loms.CreateOrder)
	case "/v1/order/info":
		return unary(ctx, raw, &lomspb.GetOrderRequest{}, b.loms.GetOrder)
	case "/v1/order/list":
		return unary(ctx, raw, &lomspb.ListOrdersRequest{}, b.loms.ListOrders)
	case "/v1/order/pay":
		return unary(ctx, raw, &lomspb.PayOrderRequest{}, b.loms.PayOrder)
	case "/v1/order/cancel":
		return unary(ctx, raw, &lomspb.CancelOrderRequest{}, b.loms.CancelOrder)

	case "/v1/notifications/telegram/bind":
		return unary(ctx, raw, &notifierpb.BindTelegramRequest{}, b.notifier.BindTelegram)

	default:
		return nil, &httpapi.Error{Code: httpapi.Unimplemented, Message: "not implemented"}
	}
}

// cartList — единственный путь, где формат ответа не совпадает с protobuf-сообщением:
// тесты ждут NDJSON-конверт {"result": {"items": [...], "total_price": N}}.
func cartList(
	ctx context.Context,
	raw json.RawMessage,
	call func(context.Context, *cartpb.ListCartRequest, ...grpc.CallOption) (*cartpb.ListCartResponse, error),
) (json.RawMessage, error) {
	var req cartpb.ListCartRequest
	if err := unmarshaler.Unmarshal(raw, &req); err != nil {
		return nil, &httpapi.Error{Code: httpapi.InvalidArgument, Message: invalidRequest}
	}
	resp, err := call(ctx, &req)
	if err != nil {
		return nil, publicError(err)
	}

	items := make([]json.RawMessage, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		encoded, mErr := marshaler.Marshal(item)
		if mErr != nil {
			return nil, fmt.Errorf("marshal cart item: %w", mErr)
		}
		items = append(items, encoded)
	}
	itemsJSON, mErr := json.Marshal(items)
	if mErr != nil {
		return nil, fmt.Errorf("marshal cart items: %w", mErr)
	}
	frame := fmt.Sprintf(`{"result":{"items":%s,"total_price":%d}}`, itemsJSON, resp.GetTotalPrice())
	return json.RawMessage(frame), nil
}

func unary[Req, Resp proto.Message](
	ctx context.Context,
	raw json.RawMessage,
	req Req,
	call func(context.Context, Req, ...grpc.CallOption) (Resp, error),
) (json.RawMessage, error) {
	if err := unmarshaler.Unmarshal(raw, req); err != nil {
		return nil, &httpapi.Error{Code: httpapi.InvalidArgument, Message: invalidRequest}
	}
	resp, err := call(ctx, req)
	if err != nil {
		return nil, publicError(err)
	}
	out, err := marshaler.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	return out, nil
}

func publicError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return &httpapi.Error{Code: int(st.Code()), Message: "internal error"}
	default:
		return &httpapi.Error{Code: int(st.Code()), Message: st.Message()}
	}
}
