package product

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/controller/converter"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
	productpb "github.com/igoroutine-courses/microservices.ecommerce.pkg/generated/loms/api/product/v1"
)

type productService interface {
	Create(ctx context.Context, p entity.Product) (string, error)
	Get(ctx context.Context, sku string) (entity.Product, error)
	Update(ctx context.Context, p entity.Product) error
	List(ctx context.Context, afterSKU string, limit int) (entity.ProductPage, error)
}

type controller struct {
	service productService
}

var _ productpb.ProductServiceServer = (*controller)(nil)

func New(service productService) *controller {
	return &controller{
		service: service,
	}
}

func (c *controller) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productpb.CreateProductResponse, error) {
	sku, err := c.service.Create(ctx, entity.Product{
		SKU:   req.GetSku(),
		Name:  req.GetName(),
		Price: req.GetPrice(),
	})
	if err != nil {
		return nil, converter.Error(err)
	}

	return &productpb.CreateProductResponse{Sku: sku}, nil
}

func (c *controller) GetProduct(ctx context.Context, req *productpb.GetProductRequest) (*productpb.GetProductResponse, error) {
	p, err := c.service.Get(ctx, req.GetSku())
	if err != nil {
		return nil, converter.Error(err)
	}

	return &productpb.GetProductResponse{
		Sku:    p.SKU,
		Name:   p.Name,
		Price:  p.Price,
		Active: p.Active,
	}, nil
}

func (c *controller) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productpb.ListProductsResponse, error) {
	page, err := c.service.List(ctx, req.GetAfterSku(), converter.PageSize(req.GetLimit()))
	if err != nil {
		return nil, converter.Error(err)
	}

	products := make([]*productpb.Product, 0, len(page.Products))
	for _, p := range page.Products {
		products = append(products, &productpb.Product{
			Sku:    p.SKU,
			Name:   p.Name,
			Price:  p.Price,
			Active: p.Active,
		})
	}

	return &productpb.ListProductsResponse{
		Products:     products,
		NextAfterSku: page.NextAfterSKU,
	}, nil
}

func (c *controller) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*emptypb.Empty, error) {
	err := c.service.Update(ctx, entity.Product{
		SKU:    req.GetSku(),
		Name:   req.GetName(),
		Price:  req.GetPrice(),
		Active: req.GetActive(),
	})
	if err != nil {
		return nil, converter.Error(err)
	}

	return &emptypb.Empty{}, nil
}
