package main

import (
	"net/url"
)

type Context struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (c *Client) GetContexts() ([]Context, error) {
	// https://circleci.com/docs/api/v2/index.html#tag/Context
	// List contexts
	u, err := url.Parse("https://circleci.com/api/v2/context")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("owner-slug", c.orgSlug)
	q.Set("owner-type", "organization")

	u.RawQuery = q.Encode()

	return get[Context](c, u, 0)
}
