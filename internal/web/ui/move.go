package ui

import (
	"fmt"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func Move(s *State) Node {
	return layout("Move", s,
		Div(
			ID("move-sse"),
			Attr("data-init", "@get('/move/partial')"),
		),
		MovePartial(s),
	)
}

func MovePartial(s *State) Node {
	isMoving := s.WorkflowRunning
	var ErrNode Node
	if s.Move.Err != nil {
		ErrNode = P(Text("Error: " + s.Move.Err.Error()))
	}
	return Div(
		ID("move-partial"),
		If(s.Move.Err != nil,
			ErrNode,
		),
		Section(
			P(Text(fmt.Sprintf("Total aips moved: %d", s.Move.TotalNumberMoved))),
			P(Text(fmt.Sprintf("Total size moved so far: %s", s.Move.TotalSizeMoved))),
		),
		Hr(),
		Section(
			If(len(s.Move.CurrentMoving) > 0,
				Div(
					H5(Text("Currently moving AIP")),
					Map(s.Move.CurrentMoving, func(aip *models.Aip) Node {
						return P(Text(aip.UUID))
					}),
				),
			),
			If(isMoving, Div(
				P(Text("Currently moving AIPs")),
				Progress(),
			)),
		),
		Button(
			If(isMoving, Disabled()),
			Style("margin-right: 0.5rem;"),
			ds.On("click", "@post('/move')"),
			Text("Move"),
		),
		Button(
			If(!isMoving, Disabled()),
			Class("secondary outline"),
			Text("Stop"),
		),
	)
}
