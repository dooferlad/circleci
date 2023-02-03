package main

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type Client struct {
	apiKey     string
	httpClient http.Client
	orgSlug    string
}

func NewClient(apiKey, orgSlug string) *Client {
	c := Client{
		apiKey:  apiKey,
		orgSlug: orgSlug,
	}

	return &c
}

func (c *Client) Request(method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// https://circleci.com/docs/api/v2/index.html#section/Authentication
	// Use a header: "Circle-Token": <key>
	req.Header.Add("Circle-Token", c.apiKey)
	return c.httpClient.Do(req)
}

type Response[T any] struct {
	NextPageToken *string `json:"next_page_token"`
	Items         []T
}

func get[T any](c *Client, url *url.URL, max int) ([]T, error) {
	var tmp []T
	var response Response[T]

	for {
		resp, err := c.Request("GET", url.String())
		if err != nil {
			return tmp, err
		}

		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return tmp, err
		}

		tmp = append(tmp, response.Items...)
		if max > 0 && len(tmp) > max {
			return tmp, nil
		}
		if response.NextPageToken != nil {
			q := url.Query()
			q.Set("page-token", *response.NextPageToken)
			url.RawQuery = q.Encode()
		} else {
			break
		}
	}

	return tmp, nil
}

func getOne[T any](c *Client, url *url.URL) (*T, error) {
	var response T
	resp, err := c.Request("GET", url.String())
	if err != nil {
		return &response, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return &response, err
}
