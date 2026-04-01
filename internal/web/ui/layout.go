package ui

import (
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
			),
			Main(Group(children)),
			Footer(),
		},
		HTMLAttrs: []Node{
			Data("theme", string(s.Theme)),
		},
	},
	)
}

func navElement(path, name string) Node {
	return Li(A(Href(path), Text(name)))
}
