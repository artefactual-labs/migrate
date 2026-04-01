package ui

import (
	"fmt"

	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func Move(s *State) Node {
	return layout("Move", s, MovePartial(s))
}

func MovePartial(s *State) Node {
	isMoving := len(s.Move.CurrentMoving) > 0
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
			// P(Text("Current Progress")),
			// Progress(
			// 	// Progress by number of AIPs
			// 	// Progress by size (total bytes)
			// 	Value("15"),
			// 	Max("100"),
			// ),
			If(isMoving, Div(
				P(Text("Currently moving AIPs")),
				Progress(),
			)),
		),
		Button(
			If(isMoving, Disabled()),
			Style("margin-right: 0.5rem;"),
			Text("Move"),
			ds.On("click", "@post('/move')"),
			// Set up the move
		),
		Button(
			If(isMoving, Disabled()),
			Class("secondary outline"),
			Text("Stop"),
		),
	)
}
