package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"

	"github.com/artefactual-labs/migrate/pkg/database/gen/models"
)

type Event struct {
	Action  Action
	Start   time.Time
	End     time.Time
	Success bool
	Details []string
}

func (e *Event) AddDetail(s string) {
	slog.Info(e.Action.String(), "msg", s)
	e.Details = append(e.Details, s)
}

func (e *Event) FormatDetails() string {
	bytes, err := json.Marshal(e.Details)
	PanicIfErr(err)
	return string(bytes)
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

func EndEvent(s AIPStatus, a *App, e Event, aip *models.Aip) {
	e.End = time.Now()
	a.UpdateAIPStatus(aip.ID, s)
	err := aip.InsertEvents(context.Background(), a.DB, EventToSetter(e))
	PanicIfErr(err)
}

func EndEventNoChange(a *App, e Event, aip *models.Aip) {
	a.reloadAIP(aip)
	EndEvent(AIPStatus(aip.Status), a, e, aip)
}

func EndEventErr(a *App, e Event, aip *models.Aip, err string) {
	e.End = time.Now()
	a.AddAIPError(aip, err)
	a.UpdateAIPStatus(aip.ID, AIPStatusFailed)
	ee := aip.InsertEvents(context.Background(), a.DB, EventToSetter(e))
	PanicIfErr(ee)
}

func EndEventErrNoFailure(a *App, e Event, aip *models.Aip, err string) {
	e.End = time.Now()
	a.AddAIPError(aip, err)
	ee := aip.InsertEvents(context.Background(), a.DB, EventToSetter(e))
	PanicIfErr(ee)
}

func EventToSetter(e Event) *models.EventSetter {
	return &models.EventSetter{
		Action:                   omit.From(e.Action.String()),
		TimeStarted:              omit.From(e.Start.String()),
		TimeEnded:                omit.From(e.End.String()),
		TotalDuration:            omitnull.From(e.Duration().String()),
		TotalDurationNanoseconds: omitnull.From(e.Duration().Nanoseconds()),
		Details:                  omitnull.From(e.FormatDetails()),
	}
}
