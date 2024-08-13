package application

import "context"

const MoveActivityName = "Move Activity"

type MoveActivityParams struct {
	UUID string `json:"uuid"`
}
type MoveActivityResult struct {
	Status string
}

func (a *App) MoveA(ctx context.Context, params MoveActivityParams) (*MoveActivityResult, error) {
	aip, err := a.GetAIPByID(params.UUID)
	if err != nil {
		return nil, err
	}
	err = move(a, aip)
	if err != nil {
		return nil, err
	}
	a.reloadAIP(aip)
	result := &MoveActivityResult{
		Status: aip.Status,
	}
	return result, nil
}
