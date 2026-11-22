package cart

type cartService interface{}

type controller struct {
	service cartService
}

func New(service cartService) *controller {
	return &controller{service: service}
}
