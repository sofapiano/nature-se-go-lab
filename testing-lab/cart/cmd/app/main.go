package main

import (
	"log"

	"github.com/igoroutine-courses/microservices.ecommerce.cart/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
