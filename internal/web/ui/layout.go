package ui

import (
	"fmt"

	lucide "github.com/eduardolat/gomponents-lucide"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

func layout(title string, s *State, children ...Node) Node {
	return HTML5(HTML5Props{
		Title:    "Migrate - " + title,
		Language: "en",
		Head: []Node{
			Link(Rel("stylesheet"), Href("/assets/css/pico.indigo.css")),
			Link(Rel("stylesheet"), Href("/assets/css/styles.css")),
			Script(Type("module"), Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.7/bundles/datastar.js")),
		},
		Body: []Node{
			Class("container"),
			Header(
				H1(Text("Migrate")),
			),
			Nav(
				Ul(
					navElement("/", "Home"),
					navElement("/aips", "AIPs"),
					navElement("/move", "Move"),
				),
				WorkflowRunning(s.WorkflowRunning),
			),
			Main(
				Div(
					ID("sse-endopoint"),
					Attr("data-init", "@get('/workflow/running')"),
				),
				Group(children),
			),
			Footer(),
		},
		HTMLAttrs: []Node{
			Data("theme", string(s.Theme)),
		},
	},
	)
}

func WorkflowRunning(running bool) Node {
	var text, iconColor string
	var icon Node
	if running {
		icon = lucide.Check()
		iconColor = "green"
		text = "Workflow running"
	} else {
		icon = lucide.TriangleAlert()
		iconColor = "orange"
		text = "No workflow running"
	}
	return Span(
		ID("workflow-running"),
		Class("with-icon align-center"),
		Style(fmt.Sprintf("color: %s", iconColor)),
		icon,
		Text(text),
	)
}

func navElement(path, name string) Node {
	return Li(A(Href(path), Text(name)))
}
