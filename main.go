package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"

	"go-downtime-api/config"
)

func main() {
	// Load variables from .env into the environment.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system environment variables")
	}

	config.ConnectDB()

	e := echo.New()

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	e.Logger.Fatal(e.Start(":8080"))
}
