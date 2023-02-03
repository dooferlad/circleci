package main

import (
	"fmt"
	"net/url"
)

type VcsInfo struct {
	VcsURL        string `json:"vcs_url"`
	Provider      string `json:"provider"`
	DefaultBranch string `json:"default_branch"`
}

type Project struct {
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	ID               string  `json:"id"`
	OrganizationName string  `json:"organization_name"`
	OrganizationSlug string  `json:"organization_slug"`
	OrganizationID   string  `json:"organization_id"`
	VCSInfo          VcsInfo `json:"vcs_info"`
	ExternalURL      string  `json:"external_url"`
}

func (c *Client) GetProject(project, jobNumber int) (*Job, error) {
	// https://circleci.com/docs/api/v2/index.html#tag/Project

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/project/%s/%s/job/%i", c.orgSlug, project, jobNumber))
	if err != nil {
		return nil, err
	}

	return getOne[Job](c, u)
}
