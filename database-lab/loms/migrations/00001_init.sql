-- +goose Up
-- Создаём изолированную схему для сервиса LOMS
CREATE SCHEMA IF NOT EXISTS loms;

-- Создаём первую таблицу (каталог товаров) как пример
CREATE TABLE IF NOT EXISTS loms.products (
    sku VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price INTEGER NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true
);

-- +goose Down
DROP TABLE IF EXISTS loms.products;
DROP SCHEMA IF EXISTS loms;