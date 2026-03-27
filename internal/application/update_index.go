package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/artefactual-labs/migrate/internal/elastic"
)

const UpdateIndexName = "update-index"

type UpdateIndexActivityParams struct {
	UUID string
}

type UpdateIndexActivityResult struct {
	Message             string
	ElasticUpdateResult any
	OriginalIndex       *elastic.ElasticAipIndexResponse
}

// UpdateIndexA Calls the elastic search API and update the index field: location with the provided name.
func (a *App) UpdateIndexA(ctx context.Context, params UpdateIndexActivityParams) (*UpdateIndexActivityResult, error) {
	if a.Config.Elastic.Host == "" {
		result := &UpdateIndexActivityResult{
			Message: "Elastic not configured, skipping index update",
		}
		return result, nil
	}

	var err error
	locationID := a.Config.StorageService.Locations.MoveTargetLocationID
	location, err := a.StorageClient.Location.Get(ctx, locationID)
	if err != nil {
		return nil, err
	}
	if location.Description == "" {
		return nil, errors.New("location empty")
	}

	elasticClient, err := elastic.NewClient(elastic.ElasticConfig{
		Version: a.Config.Elastic.Version,
		Host:    a.Config.Elastic.Host,
	})
	if err != nil {
		return nil, err
	}

	res, err := elasticClient.GetAIPByUUID(ctx, params.UUID)
	if err != nil {
		return nil, err
	}
	if len(res.Hits.Hits) != 1 {
		return nil, fmt.Errorf("expected exactly one result got: %d", len(res.Hits.Hits))
	}
	hit := res.Hits.Hits[0]
	result := &UpdateIndexActivityResult{OriginalIndex: res}

	if hit.Source.Location == location.Description {
		result.Message = "Index update not needed"
		return result, nil
	}
	response, err := elasticClient.UpdateAIPIndexLocation(ctx, hit.ID, location.Description)
	if err != nil {
		return nil, err
	}
	result.ElasticUpdateResult = response

	result.Message = "index update complete"
	return result, nil
}
