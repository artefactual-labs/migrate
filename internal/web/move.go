package web

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/web/ui"
	"github.com/labstack/echo/v5"
	"github.com/starfederation/datastar-go/datastar"
)

func (e *Endpoints) MovePage(c *echo.Context) (err error) {
	state := e.State()
	state.Move, err = moveState(c.Request().Context(), e.app)
	if err != nil {
		return err
	}
	return e.Render(c, ui.Move(state))
}

func (e *Endpoints) MoveAction(c *echo.Context) error {
	state := e.State()
	if len(e.Input) == 0 {
		state.Move = &ui.MoveState{}
		state.Move.Err = errors.New("No input file uploaded")
		return e.Render(c, ui.MovePartial(state))
	}
	e.mu.Lock()
	if !e.IsWorkflowRunning {
		go func() {
			err := application.RunWorkflowFromUUIDs(context.Background(), e.app, e.Input)
			if err != nil {
				slog.Error(err.Error())
			}
		}()
	}
	isRunning, err := e.app.IsWorkflowRunning()
	if err != nil {
		return err
	}
	e.IsWorkflowRunning = isRunning
	e.mu.Unlock()
	state.WorkflowRunning = e.IsWorkflowRunning

	moveState, err := moveState(c.Request().Context(), e.app)
	if err != nil {
		return err
	}
	state.Move = moveState
	return e.Render(c, ui.MovePartial(state))
}

func (e *Endpoints) MovePartial(c *echo.Context) error {
	sse := datastar.NewSSE(c.Response(), c.Request())
	for !sse.IsClosed() {
		state := e.State()
		moveState, err := moveState(c.Request().Context(), e.app)
		if err != nil {
			return err
		}
		state.Move = moveState
		sse.PatchElementGostar(ui.MovePartial(state))
		time.Sleep(SSELatency)
	}
	return nil
}

func moveState(ctx context.Context, app *application.App) (*ui.MoveState, error) {
	state := &ui.MoveState{}
	moving, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoving)),
	).All(ctx, app.DB)
	if err != nil {
		return nil, err
	}
	state.CurrentMoving = moving

	aipsMoved, err := models.Aips.Query(
		models.SelectWhere.Aips.Status.EQ(string(application.AIPStatusMoved)),
	).All(ctx, app.DB)
	if err != nil {
		return nil, err
	}
	var size int64
	state.TotalNumberMoved = len(aipsMoved)
	for _, aip := range aipsMoved {
		size += aip.Size.GetOrZero()
	}
	state.TotalSizeMoved = application.FormatByteSize(size)
	return state, nil
}

func MoveState() (*ui.MoveState, error) {
	return nil, nil
}
