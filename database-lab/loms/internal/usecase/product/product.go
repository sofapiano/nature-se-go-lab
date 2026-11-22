package product

type service struct{}

func New() *service {
	return &service{}
}
