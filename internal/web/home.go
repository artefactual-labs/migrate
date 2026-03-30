package web

import (
	"github.com/artefactual-labs/migrate/internal/web/ui"
	"github.com/labstack/echo/v5"
)

func (e *Endpoints) Home(c *echo.Context) error {
	return e.Render(c, ui.Home(e.State()))
}

func (e *Endpoints) State() *ui.State {
	s := &ui.State{
		SS: &ui.Service{
			Name:   "Storage Service",
			URL:    e.app.Config.StorageService.API.URL,
			Status: "Unkown",
		},
		Temporal: &ui.Service{
			Name:   "Temporal Server",
			URL:    e.app.Config.Temporal.Address,
			Status: "Unkown",
		},
		Theme: ui.ThemeLight,
	}
	return s
}
