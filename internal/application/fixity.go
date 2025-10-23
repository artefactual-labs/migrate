package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/artefactual-labs/migrate/internal/database/gen/models"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

const FixityActivityName = "fixity-activity"

type FixityActivityParams struct {
	UUID string `json:"uuid"`
}

type FixityActivityResult struct {
	Status string
}

func (a *App) FixityA(ctx context.Context, params FixityActivityParams) (*FixityActivityResult, error) {
	aip, err := a.GetAIPByID(ctx, params.UUID)
	if err != nil {
		return nil, err
	}

	if aip.FixityRun {
		return &FixityActivityResult{Status: aip.Status}, nil
	}

	if err := checkFixity(ctx, a, a.StorageClient, aip); err != nil {
		return nil, err
	}

	if err := aip.Reload(ctx, a.DB); err != nil {
		return nil, err
	}

	if aip.Status != string(AIPStatusFixityChecked) {
		return &FixityActivityResult{Status: aip.Status}, fmt.Errorf("fixity did not complete successfully, status: %s", aip.Status)
	}

	return &FixityActivityResult{Status: aip.Status}, nil
}

func checkFixity(ctx context.Context, a *App, storageClient *storage_service.API, aip *models.Aip) error {
	if aip.FixityRun {
		return nil
	}

	e := StartEvent(ActionFixity)
	e.AddDetail(fmt.Sprintf("Running fixity for: %s", aip.UUID))

	res, err := storageClient.Packages.CheckFixity(ctx, aip.UUID)
	if err != nil {
		var ssErr storage_service.SSError
		if errors.As(err, &ssErr) && ssErr.StatusCode >= 500 {
			if eventErr := EndEventErrNoFailure(ctx, a, e, aip, err.Error()); eventErr != nil {
				return eventErr
			}
			return fmt.Errorf("storage service fixity call failed: %w", err)
		}
		if eventErr := EndEventErrNoFailure(ctx, a, e, aip, err.Error()); eventErr != nil {
			return eventErr
		}
		return nil
	}

	if res.Success {
		if err := EndEvent(ctx, AIPStatusFixityChecked, a, e, aip); err != nil {
			return err
		}
		return nil
	}

	a.AddAIPError(ctx, aip, res.Message)
	for _, f := range res.Failures.Files.Changed {
		e.AddDetail("file changed: " + f)
	}
	for _, f := range res.Failures.Files.Untracked {
		e.AddDetail("file untracked: " + f)
	}
	for _, f := range res.Failures.Files.Missing {
		e.AddDetail("file missing: " + f)
	}
	if err := EndEventErr(ctx, a, e, aip, res.Message); err != nil {
		return err
	}

	return nil
}
