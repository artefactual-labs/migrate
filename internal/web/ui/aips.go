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
				Th(Text("UUID"), Scope("col")),
				Th(Text("Status"), Scope("col")),
				Th(Text("Size"), Scope("col")),
				Th(Text("Has Errors"), Scope("col")),
				Th(Text("Current Location"), Scope("col")),
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
	hasErrors := "No"
	if len(aip.R.Errors) > 1 {
		hasErrors = "Yes"
	}
	return Tr(
		Th(
			Text(aip.UUID),
			Scope("row"),
		),
		Td(Text(aip.Status)),
		Td(Text(application.FormatByteSize(aip.Size.GetOrZero()))),
		Td(Text(hasErrors)),
		Td(Text(aip.CurrentLocation.GetOrZero())),
	)
}
