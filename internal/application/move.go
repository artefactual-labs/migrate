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
	aip, err := a.GetAIPByID(ctx, params.UUID)
	if err != nil {
		return nil, err
	}
	err = move(ctx, a.logger, a, a.StorageClient, aip)
	if err != nil {
		return nil, err
	}
	if err := aip.Reload(ctx, a.DB); err != nil {
		return nil, err
	}
	result := &MoveActivityResult{
		Status: aip.Status,
	}
	return result, nil
}
