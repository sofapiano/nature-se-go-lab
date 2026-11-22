package stocks

type stocksService interface {
}
type controller struct {
	service stocksService
}

func New(service stocksService) *controller {
	return &controller{
		service: service,
	}
}
