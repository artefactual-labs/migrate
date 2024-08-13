package storage_service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

var ErrNotFound = errors.New("not found")

type API struct {
	Packages *PackageService
	Location *LocationService
}

func NewAPI(url, username, apiKey string) *API {
	client := &Client{
		Client:   http.DefaultClient,
		baseURL:  "http://" + url,
		userName: username,
		apiKey:   apiKey,
	}
	api := &API{
		Packages: &PackageService{client: client},
		Location: &LocationService{client: client},
	}
	return api
}

type Client struct {
	*http.Client
	userName string
	apiKey   string
	baseURL  string
}

type SSError struct {
	Message    string
	StatusCode int
	URL        string
	Method     string
}

func (e SSError) Error() string {
	return fmt.Sprintf("%s: (%d): url: %s method: %s", e.Message, e.StatusCode, e.URL, e.Method)
}

func NewSSError(res *http.Response, body []byte, reqURL string) error {
	SSErr := SSError{
		URL:        reqURL,
		StatusCode: res.StatusCode,
		Method:     res.Request.Method,
	}
	if res.Body != nil {
		SSErr.Message = string(body)
	} else {
		SSErr.Message = "SS API call failed"
	}
	return SSErr
}

func (c *Client) Call(method, path string, reqBody, resPayload any) error {
	var bd io.Reader
	if reqBody != nil {
		// jsonBody, err := json.Marshal(reqBody)
		// if err != nil {
		// 	return err
		// }
		bd = bytes.NewReader([]byte(reqBody.(string)))
	}

	reqUrl := c.baseURL + path
	req, err := http.NewRequest(method, reqUrl, bd)
	if err != nil {
		return err
	}

	auth := fmt.Sprintf("ApiKey %s:%s", c.userName, c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", auth)
	res, err := c.Do(req)
	if err != nil {
		return err
	}

	var body []byte
	if res.Body != nil {
		body, err = io.ReadAll(res.Body)
		if err != nil {
			slog.Error(err.Error())
			return err
		}
	}

	if res.StatusCode == 404 {
		return ErrNotFound
	} else if res.StatusCode >= 400 {
		return NewSSError(res, body, reqUrl)
	}

	if resPayload != nil && body != nil && json.Valid(body) {
		return json.Unmarshal(body, resPayload)
	}
	return nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
