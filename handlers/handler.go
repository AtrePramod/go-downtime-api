package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Handler holds what the HTTP handlers need. Passing the DB in (rather than
// reading a global) makes it easy to give the handlers a test database later.
type Handler struct {
	DB *gorm.DB
}

// New creates a Handler that uses the given database.
func New(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

// ErrorResponse is the JSON body for every error: {"error": "message"}.
type ErrorResponse struct {
	Error string `json:"error"`
}

// errorJSON writes {"error": msg} with the given status code.
func errorJSON(c echo.Context, status int, msg string) error {
	return c.JSON(status, ErrorResponse{Error: msg})
}

// internalError logs the real error for us and sends the client a generic
// 500, so database details never leak into API responses.
func internalError(c echo.Context, err error) error {
	log.Printf("%s %s: %v", c.Request().Method, c.Path(), err)
	return errorJSON(c, http.StatusInternalServerError, "internal server error")
}

// HTTPErrorHandler makes Echo's own errors (unknown route, wrong method, a
// panic recovered by middleware, ...) use the same {"error": "..."} shape.
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	msg := "internal server error"

	var he *echo.HTTPError
	if errors.As(err, &he) {
		status = he.Code
		if m, ok := he.Message.(string); ok {
			msg = m
		} else {
			msg = http.StatusText(status)
		}
	} else {
		log.Printf("%s %s: unhandled error: %v", c.Request().Method, c.Path(), err)
	}

	if err := errorJSON(c, status, msg); err != nil {
		log.Printf("failed to write error response: %v", err)
	}
}
