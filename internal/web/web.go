package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/artefactual-labs/migrate/assets"
	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/web/ui"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	gocomponents "maragu.dev/gomponents"
)

type Endpoints struct {
	app *application.App
}

func NewEndpoints(app *application.App) *Endpoints {
	return &Endpoints{
		app: app,
	}
}

func Routes(e *echo.Echo, endpoints *Endpoints) {
	e.Use(middleware.RequestLogger())

	e.StaticFS("/assets", assets.Assets)

	e.GET("/", endpoints.Home)
	e.POST("/upload", endpoints.Upload)
	e.GET("/aips", endpoints.Aips)
	e.GET("/aips/search", endpoints.AipsSearch)
}

func (e *Endpoints) AipsSearch(c *echo.Context) error {
	type aipsearch struct {
		Search string `json:"aip-search"`
	}
	var payload aipsearch
	query := c.QueryParam("datastar")
	err := json.Unmarshal([]byte(query), &payload)
	if err != nil {
		return err
	}

	res, err := models.Aips.Query(
		models.SelectWhere.Aips.UUID.Like(fmt.Sprintf("%%%s%%", payload.Search)),
	).All(context.Background(), e.app.DB)
	if err != nil {
		return err
	}
	return e.Render(c, ui.AipsTable(res))
}

func (e *Endpoints) Aips(c *echo.Context) error {
	res, err := models.Aips.Query().All(context.Background(), e.app.DB)
	if err != nil {
		return err
	}
	return e.Render(c, ui.Aips(res, e.State()))
}

func (e *Endpoints) Upload(c *echo.Context) error {
	type InputFile struct {
		Name     string `json:"name"`
		Contents string `json:"contents"`
		Mime     string `json:"mime"`
	}
	type UploadPayload struct {
		InputFile []InputFile `json:"input-file"`
	}
	var payload UploadPayload
	err := c.Bind(&payload)
	if err != nil {
		return err
	}
	if len(payload.InputFile) != 1 {
		return fmt.Errorf("exactly one file is allowed to be uploaded, got: %d", len(payload.InputFile))
	}
	file := payload.InputFile[0]
	if file.Mime != "text/plain" {
		return fmt.Errorf("only .txt files accepted")
	}

	raw := file.Contents
	content, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	slog.Info("file uploaded", "name", file.Name)

	slog.Info("validating UUIDs")
	uuids, err := application.ValidateUniqueUUIDs(lines)
	if err != nil {
		return err
	}

	state := e.State()
	state.Input = uuids
	return e.Render(c, ui.Home(state))
}

func (e *Endpoints) Render(c *echo.Context, page gocomponents.Node) error {
	return page.Render(c.Response())
}
