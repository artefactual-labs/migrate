package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
)

type Action struct {
	name string
}

func (a Action) String() string {
	return a.name
}

var (
	ActionFind      = Action{"find"}
	ActionMove      = Action{"move"}
	ActionReplicate = Action{"Replicate"}
	ActionIndex     = Action{"index"}
)

type Event struct {
	Action  Action
	Start   time.Time
	End     time.Time
	Success bool
	Details []string
}

func (e *Event) AddDetail(s string) {
	e.Details = append(e.Details, s)
}

func (e *Event) FormatDetails() (string, error) {
	bytes, err := json.Marshal(e.Details)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (e *Event) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

func StartEvent(a Action) Event {
	return Event{
		Action: a,
		Start:  time.Now(),
	}
}

func EndEvent(ctx context.Context, s AIPStatus, a *App, e Event, aip *models.Aip) error {
	e.End = time.Now()
	if err := a.UpdateAIPStatus(ctx, aip.ID, s); err != nil {
		return err
	}
	setter, err := EventToSetter(e)
	if err != nil {
		return err
	}
	return aip.InsertEvents(ctx, a.DB, setter)
}

func EndEventNoChange(ctx context.Context, a *App, e Event, aip *models.Aip) error {
	if err := aip.Reload(ctx, a.DB); err != nil {
		return err
	}
	return EndEvent(ctx, AIPStatus(aip.Status), a, e, aip)
}

func EndEventErr(ctx context.Context, a *App, e Event, aip *models.Aip, eventErr string) error {
	e.End = time.Now()
	a.AddAIPError(ctx, aip, eventErr)
	if err := a.UpdateAIPStatus(ctx, aip.ID, AIPStatusFailed); err != nil {
		return err
	}
	setter, err := EventToSetter(e)
	if err != nil {
		return err
	}
	return aip.InsertEvents(ctx, a.DB, setter)
}

func EventToSetter(e Event) (*models.EventSetter, error) {
	formatDetails, err := e.FormatDetails()
	if err != nil {
		return nil, err
	}
	return &models.EventSetter{
		Action:                   omit.From(e.Action.String()),
		TimeStarted:              omit.From(e.Start.String()),
		TimeEnded:                omit.From(e.End.String()),
		TotalDuration:            omitnull.From(e.Duration().String()),
		TotalDurationNanoseconds: omitnull.From(e.Duration().Nanoseconds()),
		Details:                  omitnull.From(formatDetails),
	}, nil
}
