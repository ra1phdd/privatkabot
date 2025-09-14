package main

import (
	"log"
	"privatkabot/internal/pkg/app"
)

func main() {
	err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	select {}
}
