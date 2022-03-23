package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/starkandwayne/scheduler-for-ocf/core"
	"github.com/starkandwayne/scheduler-for-ocf/http/auth"
)

type callCollection struct {
	Pagination *pagination  `json:"pagination"`
	Resources  []*core.Call `json:"resources"`
}

func AllCalls(e *echo.Echo, services *core.Services) {
	// Get all Calls within space
	// GET /calls?space_guid=string
	e.GET("/calls", func(c echo.Context) error {
		if auth.Verify(c) != nil {
			return c.JSON(http.StatusUnauthorized, "")
		}

		spaceGUID := c.QueryParam("space_guid")

		calls := services.Calls.InSpace(spaceGUID)

		output := &callCollection{
			Resources: calls,
			Pagination: &pagination{
				TotalPages:   1,
				TotalResults: len(calls),
				First:        &pageref{Href: "first"},
				Last:         &pageref{Href: "last"},
				Next:         &pageref{Href: "next"},
				Previous:     &pageref{Href: "previous"},
			},
		}

		return c.JSON(
			http.StatusOK,
			output,
		)
	})
}