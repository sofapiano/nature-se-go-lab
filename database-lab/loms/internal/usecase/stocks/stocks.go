package stocks

type (
	stocksRepository interface{}

	productsRepository interface{}
)

type service struct {
	stocks   stocksRepository
	products productsRepository
}

func New(stocks stocksRepository, products productsRepository) *service {
	return &service{
		stocks:   stocks,
		products: products,
	}
}
