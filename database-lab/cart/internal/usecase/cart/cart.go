package cart

type cartRepository interface{}

type checkoutRepository interface{}

type catalogClient interface{}

type lomsClient interface{}
type service struct{}

func New(carts cartRepository, attempts checkoutRepository, catalog catalogClient, loms lomsClient) *service {
	return &service{}
}
