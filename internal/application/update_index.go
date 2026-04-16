package application

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/artefactual-labs/migrate/internal/elastic"
	"github.com/artefactual-labs/migrate/internal/storage_service"
)

const UpdateIndexName = "update-index"

type UpdateIndexActivityParams struct {
	UUID   string
	DryRun bool
}

type UpdateIndexActivityResult struct {
	Message             string
	ElasticUpdateResult any
	OriginalIndex       *elastic.ElasticAipIndexResponse
	UpdatedFields       []string
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
	if location.Path == "" {
		return nil, errors.New("location path empty")
	}

	result := &UpdateIndexActivityResult{}
	pkg, err := a.StorageClient.Packages.GetByID(ctx, params.UUID)
	if err != nil {
		return nil, err
	}
	if !packageReadyForIndexUpdate(pkg, locationID) {
		result.Message = fmt.Sprintf(
			"Elasticsearch update skipped: package status is %s at location %s",
			pkg.Status,
			pkg.CurrentLocation,
		)
		return result, nil
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
	result.OriginalIndex = res
	filePath := buildIndexFilePath(location.Path, pkg.CurrentPath, hit.Source.FilePath)
	if hit.Source.Location != location.Description {
		result.UpdatedFields = append(result.UpdatedFields, "location")
	}
	if hit.Source.FilePath != filePath {
		result.UpdatedFields = append(result.UpdatedFields, "filePath")
	}

	if len(result.UpdatedFields) == 0 {
		result.Message = "Elasticsearch update not needed: location and filePath already matched target"
		return result, nil
	}
	if params.DryRun {
		result.Message = formatUpdateIndexMessage(result.UpdatedFields, true)
		return result, nil
	}
	response, err := elasticClient.UpdateAIPIndex(ctx, hit.ID, location.Description, filePath)
	if err != nil {
		return nil, err
	}
	result.ElasticUpdateResult = response

	result.Message = formatUpdateIndexMessage(result.UpdatedFields, false)
	return result, nil
}

func formatUpdateIndexMessage(updatedFields []string, dryRun bool) string {
	prefix := "Updated Elasticsearch fields: "
	if dryRun {
		prefix = "Dry run: would update Elasticsearch fields: "
	}

	return prefix + strings.Join(updatedFields, ", ")
}

func packageReadyForIndexUpdate(pkg *storage_service.Package, targetLocationID string) bool {
	if pkg == nil {
		return false
	}

	return pkg.Status == "UPLOADED" && strings.Contains(pkg.CurrentLocation, targetLocationID)
}

func buildIndexFilePath(locationPath, currentPath, indexedPath string) string {
	cleanLocationPath := strings.TrimRight(locationPath, "/")
	cleanCurrentPath := strings.TrimLeft(currentPath, "/")
	if cleanCurrentPath != "" {
		return path.Clean(path.Join(cleanLocationPath, cleanCurrentPath))
	}

	base := path.Base(indexedPath)
	if base == "." || base == "/" || base == "" {
		return path.Clean(cleanLocationPath)
	}

	return path.Clean(path.Join(cleanLocationPath, base))
}
