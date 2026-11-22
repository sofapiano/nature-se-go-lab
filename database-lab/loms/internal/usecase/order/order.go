package order

type (
	orderRepository    interface{}
	productRepository  interface{}
	stocksRepository   interface{}
	billingClient      interface{}
	notificationClient interface{}
)

type service struct {
	orders   orderRepository
	products productRepository
	stocks   stocksRepository
	billing  billingClient
	notifier notificationClient
}

func New(
	orders orderRepository,
	products productRepository,
	stocks stocksRepository,
	billing billingClient,
	notifier notificationClient,
) *service {
	return &service{
		orders:   orders,
		products: products,
		stocks:   stocks,
		billing:  billing,
		notifier: notifier,
	}
}
