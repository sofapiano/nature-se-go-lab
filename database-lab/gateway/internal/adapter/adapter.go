package adapter

import (
	"context"
	"encoding/json"
)

type backend struct{}

const invalidRequest = "invalid request"

type Backend interface {
	Call(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

var _ Backend = (*backend)(nil)

func New(cartAddress, lomsAddress, notifierAddress string) (*backend, func(), error) {
	panic("not implemented")
}

func (b *backend) Call(ctx context.Context, path string, raw json.RawMessage) (json.RawMessage, error) {
	// Example:
	// switch path {
	// case "/v1/cart/item/add":
	//     return unary(ctx, raw, &cart.AddItemRequest{}, b.cart.AddItem)
	// case "/v1/cart/item/set":
	//     return unary(ctx, raw, &cart.SetItemQuantityRequest{}, b.cart.SetItemQuantity)
	// case "/v1/cart/item/delete":
	//     return unary(ctx, raw, &cart.DeleteItemRequest{}, b.cart.DeleteItem)
	// }
	panic("not implemented")
}
