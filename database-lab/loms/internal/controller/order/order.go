package order

type orderService interface {
}
type controller struct {
	service orderService
}

func New(service orderService) *controller {
	return &controller{
		service: service,
	}
}
