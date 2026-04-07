package ui

import (
	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/google/uuid"
)

type State struct {
	Input           []uuid.UUID
	Temporal        *Service
	SS              *Service
	Theme           Theme
	Move            *MoveState
	AIP             *models.Aip
	WorkflowRunning bool
}

type MoveState struct {
	CurrentMoving    []*models.Aip
	TotalNumberMoved int
	TotalSizeMoved   string
	Err              error
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
