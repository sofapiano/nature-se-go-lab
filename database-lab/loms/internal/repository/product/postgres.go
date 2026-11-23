package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/db"
	"github.com/igoroutine-courses/microservices.ecommerce.loms/internal/entity"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *postgresRepository {
	return &postgresRepository{pool: pool}
}

// Create добавляет товар.
func (r *postgresRepository) Create(ctx context.Context, p entity.Product) error {
	exec := db.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, `
		INSERT INTO loms.products (sku, name, price, active)
		VALUES ($1, $2, $3, $4)
	`, p.SKU, p.Name, p.Price, p.Active)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return fmt.Errorf("product %q already exists: %w", p.SKU, entity.ErrConflict)
		}
		return fmt.Errorf("create product: %w", err)
	}

	return nil
}

// Get возвращает товар по SKU.
func (r *postgresRepository) Get(ctx context.Context, sku string) (entity.Product, error) {
	exec := db.GetExecutor(ctx, r.pool)

	var p entity.Product
	err := exec.QueryRow(ctx, `
		SELECT sku, name, price, active
		FROM loms.products
		WHERE sku = $1
	`, sku).Scan(&p.SKU, &p.Name, &p.Price, &p.Active)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Product{}, fmt.Errorf("product %q: %w", sku, entity.ErrNotFound)
		}
		return entity.Product{}, fmt.Errorf("get product: %w", err)
	}

	return p, nil
}

// Update заменяет данные существующего товара.
func (r *postgresRepository) Update(ctx context.Context, p entity.Product) error {
	exec := db.GetExecutor(ctx, r.pool)

	tag, err := exec.Exec(ctx, `
		UPDATE loms.products
		SET name = $2, price = $3, active = $4
		WHERE sku = $1
	`, p.SKU, p.Name, p.Price, p.Active)

	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("product %q: %w", p.SKU, entity.ErrNotFound)
	}

	return nil
}

// List возвращает до limit товаров с SKU строго больше afterSKU, по возрастанию.
func (r *postgresRepository) List(ctx context.Context, afterSKU string, limit int) ([]entity.Product, error) {
	exec := db.GetExecutor(ctx, r.pool)

	if limit <= 0 {
		limit = 10
	}

	rows, err := exec.Query(ctx, `
		SELECT sku, name, price, active
		FROM loms.products
		WHERE sku > $1 AND active = true
		ORDER BY sku ASC
		LIMIT $2
	`, afterSKU, limit)

	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]entity.Product, 0, limit)
	for rows.Next() {
		var p entity.Product
		if err := rows.Scan(&p.SKU, &p.Name, &p.Price, &p.Active); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return products, nil
}
