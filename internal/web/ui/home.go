package ui

import (
	"fmt"

	"github.com/google/uuid"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func Home(state *State) Node {
	/*
		TODOS:
			- Have a way for adding more input.txt
			- AIPs Page
				- Show all AIPs
				- Have a search bar, search by UUID
				- When clicking in one AIP, get info on it.
			- Move
				- Total SIZE so far.
				- Total SIZE Total.
				- List All AIPs that have been succesfully Moved
				- Show status of currently moving AIP
					- Poll the backend fot this information, if AIP is moving.
					- Every tick in the backoff workflow will send back a signal to the Front-End
	*/
	return layout("Home", state,
		Div(
			H2(Text("Configuration")),
			Div(
				Class("grid"),
				inputStatus(state),
				services(state),
			),
		),
	)
}

func inputStatus(s *State) Node {
	if len(s.Input) == 0 {
		return Article(
			Header(Text("Input")),
			P(Text("Only valid UUIDs are accepted. They will also be deduplicated")),
			Label(
				Input(
					Type("file"),
					Accept(".txt"),
					ds.Bind("input-file"),
				),
				Small(Text("only .txt files accepted - max size 1 MIB")),
			),
			Button(
				Type("submit"),
				Text("Upload Input file"),
				ds.On("click", "@post('/upload')"),
			),
		)
	} else {
		return Article(
			Header(Text(fmt.Sprintf("Input file has %d uuids", len(s.Input)))),
			Map(s.Input, func(id uuid.UUID) Node {
				return P(Text(id.String()))
			}),
		)
	}
}

func services(s *State) Node {
	services := []Node{}
	if s.Temporal != nil {
		services = append(services, srv(s.Temporal))
	}
	if s.SS != nil {
		services = append(services, srv(s.SS))
	}
	return Div(services...)
}

func srv(s *Service) Node {
	if s == nil {
		return nil
	}
	return Article(
		Header(Text(s.Name)),
		P(Text(fmt.Sprintf("URL: %s", s.URL))),
		P(Text(fmt.Sprintf("Status: %s", s.Status))),
	)
}
