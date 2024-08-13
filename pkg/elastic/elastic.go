package elastic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type API struct {
	url string
}

func New(url string) *API {
	return &API{url}
}

func (a *API) GetAIPIndex(uuid string) (*AIPIndex, error) {
	var index *SourcePayload
	type Term struct {
		UUID string `json:"uuid"`
	}
	type Query struct {
		Term Term `json:"term"`
	}
	type Payload struct {
		Query Query `json:"query"`
	}
	body := Payload{Query: Query{Term{uuid}}}
	err := a.do(http.MethodGet, "/aips/_search", body, &index)
	if err != nil {
		return nil, err
	}
	return index.Hits.Hits[0], nil
}

func (a *API) UpdateAIPIndex(id, filePath, location string) error {
	var res *UpdateResponse
	type Doc struct {
		FilePath string `json:"filePath"`
		Location string `json:"location"`
	}
	type Payload struct {
		Doc Doc `json:"doc"`
	}
	path := fmt.Sprintf("/aips/_doc/%s/_update", id)
	body := Payload{Doc: Doc{filePath, location}}
	err := a.do(http.MethodPost, path, body, &res)
	if err != nil {
		return err
	}
	if res.Shards.Total != 2 && res.Shards.Successful != 1 && res.Shards.Failed > 0 {
		slog.Error("update to index failed", "res", res)
		return errors.New("update to index failed")
	}
	return nil
}

func (a *API) do(method, path string, reqBody, resPayload any) error {
	var bd io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		bd = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, a.url+path, bd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var body []byte
	if res.Body != nil {
		body, err = io.ReadAll(res.Body)
		if err != nil {
			return err
		}
	}

	if res.StatusCode >= 400 {
		slog.Error("error with elastic search", "body", string(body))
		return errors.New("error: " + res.Status)
	}
	if resPayload != nil && body != nil && json.Valid(body) {
		return json.Unmarshal(body, resPayload)
	}
	return nil
}

type SourcePayload struct {
	Took     int    `json:"took"`
	TimedOut bool   `json:"timed_out"`
	Shards   Shards `json:"_shards"`
	Hits     Hits   `json:"hits"`
}

type Shards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type Source struct {
	UUID             string  `json:"uuid"`
	Name             string  `json:"name"`
	FilePath         string  `json:"filePath"`
	Size             float64 `json:"size"`
	FileCount        int     `json:"file_count"`
	Origin           string  `json:"origin"`
	Created          int     `json:"created"`
	Aicid            any     `json:"AICID"`
	IsPartOf         any     `json:"isPartOf"`
	CountAIPsinAIC   any     `json:"countAIPsinAIC"`
	Identifiers      []any   `json:"identifiers"`
	TransferMetadata []any   `json:"transferMetadata"`
	Encrypted        bool    `json:"encrypted"`
	Accessionids     []any   `json:"accessionids"`
	Status           string  `json:"status"`
	Location         string  `json:"location"`
	Path             string  `json:"path"`
}

type AIPIndex struct {
	Index  string  `json:"_index"`
	Type   string  `json:"_type"`
	ID     string  `json:"_id"`
	Score  float64 `json:"_score"`
	Source Source  `json:"_source"`
}

type Hits struct {
	Total    int         `json:"total"`
	MaxScore float64     `json:"max_score"`
	Hits     []*AIPIndex `json:"hits"`
}

type UpdateResponse struct {
	Index       string `json:"_index"`
	Type        string `json:"_type"`
	ID          string `json:"_id"`
	Version     int    `json:"_version"`
	Result      string `json:"result"`
	Shards      Info   `json:"_shards"`
	SeqNo       int    `json:"_seq_no"`
	PrimaryTerm int    `json:"_primary_term"`
}

type Info struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}
