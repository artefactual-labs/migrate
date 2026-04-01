package ui

import (
	"fmt"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func Aips(aipSlice []*models.Aip, s *State) Node {
	return layout("Aips", s, AipsPage(aipSlice))
}

func AipsPage(aips []*models.Aip) Node {
	contents := Div(
		Div(
			Input(
				Type("search"),
				Name("aip-search"),
				Placeholder("search AIPs by UUID"),
				ds.Bind("aip-search"),
				ds.On("input__debounce.200ms", "@get('/aips/search')"),
			),
		),
		AipsTable(aips),
	)
	return contents
}

func AipsTable(aips []*models.Aip) Node {
	if len(aips) < 1 {
		return Div(
			ID("aips-table"),
			P(Text("No aips found")),
		)
	}

	return Div(
		ID("aips-table"),
		H5(Text(fmt.Sprintf("Total: %d", len(aips)))),
		Table(
			Class("striped"),
			THead(
				Th(Text("Status"), Scope("col")),
				Th(Text("UUID"), Scope("col")),
				Th(Text("Size"), Scope("col")),
			),
			TBody(
				Map(aips, func(aip *models.Aip) Node {
					return aipNode(aip)
				}),
			),
		),
	)
}

func aipNode(aip *models.Aip) Node {
	return Tr(
		Th(
			Text(aip.Status),
			Scope("row"),
		),
		Td(
			A(
				Href(fmt.Sprintf("/aips/%s", aip.UUID)),
				Text(aip.UUID),
			),
		),
		Td(Text(application.FormatByteSize(aip.Size.GetOrZero()))),
	)
}

func AIP(s *State) Node {
	if s.AIP == nil {
		return layout("AIP", s, Div(
			P(Text("Server error: AIP is nil or not found")),
		))
	}

	return layout("AIP", s, Div(
		Div(
			Class("grid"),
			Article(
				Header(Text("AIP")),
				keyVal("UUID", s.AIP.UUID),
				keyVal("Status", s.AIP.Status),
				keyVal("Size", application.FormatByteSize(s.AIP.Size.GetOrZero())),
				keyVal("Last known Location", s.AIP.CurrentLocation.GetOrZero()),
				keyVal("Location UUID", s.AIP.LocationUUID.GetOrZero()),
				keyVal("Moved", application.FormatBool(s.AIP.Moved)),
				keyVal("Fixity Done", application.FormatBool(s.AIP.FixityRun)),
				keyVal("Replicated", application.FormatBool(s.AIP.Replicated)),
				keyVal("Re-Indexed", application.FormatBool(s.AIP.ReIndexed)),
			),
			Article(
				Header(Text("Errors")),
				Map(s.AIP.R.Errors, func(e *models.Error) Node {
					return Group([]Node{
						keyVal("Message", e.MSG),
						keyVal("Details", e.Details.GetOrZero()),
					})
				}),
				If(len(s.AIP.R.Errors) == 0, P(Text("No errors"))),
			),
		),
		Article(
			Header(Text("Events")),
			Map(s.AIP.R.Events, func(e *models.Event) Node {
				return Group([]Node{
					H3(Text(e.Action)),
					keyVal("Time Stared", e.TimeStarted),
					keyVal("Time Ended", e.TimeEnded),
					keyVal("Total Duration", e.TotalDuration.GetOrZero()),
					keyVal("Details", e.Details.GetOrZero()),
					Br(),
				})
			}),
		),
	))
}

func keyVal(key, val string) Node {
	if val == "" {
		val = "unknown"
	}
	return P(
		Strong(Text(key+": ")),
		Text(val),
	)
}
