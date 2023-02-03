package main

import (
	"fmt"
	"net/url"
	"time"
)

type Pipeline struct {
	ID                string            `json:"id"`
	Errors            []Error           `json:"errors"`
	ProjectSlug       string            `json:"project_slug"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Number            int64             `json:"number"`
	TriggerParameters TriggerParameters `json:"trigger_parameters"`
	State             string            `json:"state"`
	CreatedAt         time.Time         `json:"created_at"`
	Trigger           Trigger           `json:"trigger"`
	Vcs               Vcs               `json:"vcs"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
type TriggerParameters struct {
	Property1 string `json:"property1"`
	Property2 string `json:"property2"`
}
type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}
type Trigger struct {
	Type       string    `json:"type"`
	ReceivedAt time.Time `json:"received_at"`
	Actor      Actor     `json:"actor"`
}
type Commit struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}
type Vcs struct {
	ProviderName        string `json:"provider_name"`
	TargetRepositoryURL string `json:"target_repository_url"`
	Branch              string `json:"branch"`
	ReviewID            string `json:"review_id"`
	ReviewURL           string `json:"review_url"`
	Revision            string `json:"revision"`
	Tag                 string `json:"tag"`
	Commit              Commit `json:"commit"`
	OriginRepositoryURL string `json:"origin_repository_url"`
}

func (c *Client) GetOrgPipelines(max int) ([]Pipeline, error) {
	// https://circleci.com/docs/api/v2/index.html#operation/listPipelines

	u, err := url.Parse("https://circleci.com/api/v2/pipeline")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("org-slug", c.orgSlug)

	u.RawQuery = q.Encode()

	return get[Pipeline](c, u, max)
}

func (c *Client) GetProjectPipelines(project, branch string, max int) ([]Pipeline, error) {
	// https://circleci.com/docs/api/v2/index.html#operation/listPipelinesForProject

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/project/%s/%s/pipeline", c.orgSlug, project))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if branch != "" {
		q.Set("branch", branch)
	}
	u.RawQuery = q.Encode()

	return get[Pipeline](c, u, max)
}

func (c *Client) GetPipelineWorkflows(id string, max int) ([]Workflow, error) {
	// https://circleci.com/docs/api/v2/index.html#operation/listPipelinesForProject

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/pipeline/%s/workflow", id))
	if err != nil {
		return nil, err
	}

	return get[Workflow](c, u, max)
}
