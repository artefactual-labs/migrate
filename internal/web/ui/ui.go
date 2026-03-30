package ui

import "github.com/google/uuid"

type State struct {
	Input    []uuid.UUID
	Temporal *Service
	SS       *Service
	Theme    Theme
}

type Service struct {
	Name   string
	URL    string
	Status string
}

type Theme string

const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)
