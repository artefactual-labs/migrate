package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	es6 "github.com/elastic/go-elasticsearch/v6"
	es8 "github.com/elastic/go-elasticsearch/v8"
)

type ElasticClient interface {
	GetAIPByUUID(ctx context.Context, uuid string) (*ElasticAipIndexResponse, error)
	UpdateAIPIndexLocation(ctx context.Context, id, location string) (map[string]any, error)
}

type ElasticConfig struct {
	Version string
	Host    string
}

func NewClient(config ElasticConfig) (ElasticClient, error) {
	switch config.Version {
	case "v6":
		return NewV6(config.Host)
	case "v8":
		return NewV8(config.Host)
	default:
		return nil, fmt.Errorf("version not supported: %s", config.Version)
	}
}

type ElasticV6 struct {
	client *es6.Client
}

func (e ElasticV6) GetAIPByUUID(ctx context.Context, uuid string) (*ElasticAipIndexResponse, error) {
	var buf bytes.Buffer
	q := QueryAIPUUID_V6{}
	q.Query.Term.UUID = uuid
	err := json.NewEncoder(&buf).Encode(q)
	if err != nil {
		return nil, err
	}
	res, err := e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex("aips"),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	var elasticRes ElasticAipIndexResponse
	err = unmarshal(res.Body, &elasticRes)
	if err != nil {
		return nil, err
	}
	return &elasticRes, nil
}

func (e ElasticV6) UpdateAIPIndexLocation(ctx context.Context, id, location string) (map[string]any, error) {
	doc := struct {
		Doc struct {
			Location string `json:"location"`
		} `json:"doc"`
	}{}
	doc.Doc.Location = location
	data, err := json.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	res, err := e.client.Update(
		"aips",
		id,
		reader,
	)
	if err != nil {
		return nil, err
	}

	m := map[string]any{}
	return m, unmarshal(res.Body, &m)
}

func NewV6(host string) (ElasticClient, error) {
	c, err := es6.NewClient(es6.Config{
		Addresses: []string{host},
	})
	return ElasticV6{client: c}, err
}

func unmarshal(r io.ReadCloser, v any) error {
	//nolint:errcheck
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &v)
}

type ElasticV8 struct {
	client *es8.Client
}

func NewV8(host string) (ElasticClient, error) {
	c, err := es8.NewClient(es8.Config{
		Addresses: []string{host},
	})
	return ElasticV8{client: c}, err
}

func (e ElasticV8) GetAIPByUUID(ctx context.Context, uuid string) (*ElasticAipIndexResponse, error) {
	var buf bytes.Buffer
	q := QueryAIPUUID_V6{}
	q.Query.Term.UUID = uuid
	err := json.NewEncoder(&buf).Encode(q)
	if err != nil {
		return nil, err
	}
	res, err := e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex("aips"),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	var elasticRes ElasticAipIndexResponse
	err = unmarshal(res.Body, &elasticRes)
	if err != nil {
		return nil, err
	}
	return &elasticRes, nil
}

func (e ElasticV8) UpdateAIPIndexLocation(ctx context.Context, id, location string) (map[string]any, error) {
	doc := struct {
		Doc struct {
			Location string `json:"location"`
		} `json:"doc"`
	}{}
	doc.Doc.Location = location
	data, err := json.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	res, err := e.client.Update(
		"aips",
		id,
		reader,
	)
	if err != nil {
		return nil, err
	}

	m := map[string]any{}
	return m, unmarshal(res.Body, &m)
}
