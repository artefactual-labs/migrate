package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/artefactual-labs/migrate/assets"
	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/web/ui"
	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	gocomponents "maragu.dev/gomponents"
)

type Endpoints struct {
	app   *application.App
	Input []uuid.UUID
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
	e.GET("/aips/:id", endpoints.GetAIP)
	e.GET("/move", endpoints.MovePage)
	e.POST("/move", endpoints.MoveAction)
}

func (e *Endpoints) GetAIP(c *echo.Context) error {
	id := c.Param("id")
	aip, err := e.app.GetAIPByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	aip.LoadErrors(c.Request().Context(), e.app.DB)
	aip.LoadEvents(c.Request().Context(), e.app.DB)
	state := e.State()
	state.AIP = aip
	return e.Render(c, ui.AIP(state))
}

func (e *Endpoints) MoveAction(c *echo.Context) error {
	state := e.State()
	state.Move = &ui.MoveState{}
	if len(e.Input) == 0 {
		state.Move.Err = errors.New("No input file uploaded")
		return e.Render(c, ui.MovePartial(state))
	}
	var mu sync.Mutex
	mu.Lock()
	for _, id := range e.Input {
		workflowID := fmt.Sprintf("AIP_Move_%s", id.String())
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: e.app.Config.Temporal.TaskQueue,
			// Allow duplicate execution only when the previous run closed
			// unsuccessfully. This prevents two healthy runs from processing the
			// same AIP at the same time.
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		}
		params := application.MoveWorkflowParams{
			UUID: id,
		}
		e.app.Tc.ExecuteWorkflow(c.Request().Context(), options, application.MoveWorkflowName, params)
		slog.Info("Launched workflow", "id", workflowID)
	}
	mu.Unlock()

	moving, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoving)),
	).All(c.Request().Context(), e.app.DB)
	if err != nil {
		return err
	}
	state.Move.CurrentMoving = moving

	aipsMoved, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoved)),
	).All(c.Request().Context(), e.app.DB)
	if err != nil {
		return err
	}
	var size int64
	state.Move.TotalNumberMoved = len(aipsMoved)
	for _, aip := range aipsMoved {
		size += aip.Size.GetOrZero()
	}
	state.Move.TotalSizeMoved = application.FormatByteSize(size)
	return e.Render(c, ui.MovePartial(state))
}

func (e *Endpoints) MovePage(c *echo.Context) error {
	state := e.State()
	state.Move = &ui.MoveState{}

	moving, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoving)),
	).All(c.Request().Context(), e.app.DB)
	if err != nil {
		return err
	}
	state.Move.CurrentMoving = moving

	aipsMoved, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoved)),
	).All(c.Request().Context(), e.app.DB)
	if err != nil {
		return err
	}
	var size int64
	state.Move.TotalNumberMoved = len(aipsMoved)
	for _, aip := range aipsMoved {
		size += aip.Size.GetOrZero()
	}
	state.Move.TotalSizeMoved = application.FormatByteSize(size)

	return e.Render(c, ui.Move(state))
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

	var mu sync.Mutex
	mu.Lock()
	e.Input = uuids
	mu.Unlock()
	return e.Render(c, ui.Home(e.State()))
}

func (e *Endpoints) Render(c *echo.Context, page gocomponents.Node) error {
	return page.Render(c.Response())
}
