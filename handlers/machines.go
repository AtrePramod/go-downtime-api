package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"go-downtime-api/models"
)

// CreateMachineRequest is the body for POST /machines.
type CreateMachineRequest struct {
	Name string `json:"name"`
	Line string `json:"line"`
}

// CreateMachine handles POST /machines.
func (h *Handler) CreateMachine(c echo.Context) error {
	var req CreateMachineRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest, "invalid JSON body")
	}

	if err := req.Validate(); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	machine := models.Machine{Name: req.Name, Line: req.Line}
	if err := h.DB.Create(&machine).Error; err != nil {
		// Machine names have a unique index.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorJSON(c, http.StatusConflict, "a machine with this name already exists")
		}
		return internalError(c, err)
	}

	return c.JSON(http.StatusCreated, machine)
}

// ListMachines handles GET /machines.
func (h *Handler) ListMachines(c echo.Context) error {
	machines := []models.Machine{} // non-nil, so an empty result is [] not null
	if err := h.DB.Order("id").Find(&machines).Error; err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, machines)
}
