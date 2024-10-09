package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
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
	req.Header.Add("Accept", "application/json")
	return c.httpClient.Do(req)
}

type Response[T any] struct {
	NextPageToken *string `json:"next_page_token"`
	Items         []T
	Message       *string `json:"message"`
}

type APIMessage struct {
	Message *string `json:"message"`
}

func get[T any](c *Client, url *url.URL, max int) ([]T, error) {
	var tmp []T
	var response Response[T]

	for {
		resp, err := c.Request("GET", url.String())
		if err != nil {
			logrus.Error("get failed to ", url.String())
			return tmp, err
		}

		if resp.StatusCode >= 400 {
			return tmp, errors.New(fmt.Sprint("HTTP error: ", resp.StatusCode))
		}

		var buffer bytes.Buffer
		buffer.ReadFrom(resp.Body)

		if err = json.NewDecoder(bytes.NewReader(buffer.Bytes())).Decode(&response); err != nil {
			var message APIMessage
			json.NewDecoder(bytes.NewReader(buffer.Bytes())).Decode(&message)

			logrus.Error("JSON decode error: ", err, " accessing ", url.String(), " ", buffer.String())
			return tmp, err
		}

		if response.Message != nil && *response.Message == "Pipeline not found" {
			continue
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
