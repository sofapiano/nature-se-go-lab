package product

type productService interface{}
type controller struct {
	service productService
}

func New(service productService) *controller {
	return &controller{
		service: service,
	}
}
