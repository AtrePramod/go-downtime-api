package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"

	"go-downtime-api/config"
	"go-downtime-api/handlers"
)

func main() {
	// Load variables from .env into the environment.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system environment variables")
	}

	config.ConnectDB()
	config.Migrate()

	e := echo.New()
	e.HTTPErrorHandler = handlers.HTTPErrorHandler

	h := handlers.New(config.DB)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	e.POST("/machines", h.CreateMachine)
	e.GET("/machines", h.ListMachines)

	e.POST("/downtime", h.CreateDowntime)
	e.GET("/downtime", h.ListDowntime)

	e.GET("/reports/daily", h.DailyReport)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}
