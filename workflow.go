package main

import (
	"fmt"
	"net/url"
	"time"
)

type Workflow struct {
	PipelineID     string    `json:"pipeline_id"`
	CanceledBy     string    `json:"canceled_by"`
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ProjectSlug    string    `json:"project_slug"`
	ErroredBy      string    `json:"errored_by"`
	Tag            string    `json:"tag"`
	Status         string    `json:"status"`
	StartedBy      string    `json:"started_by"`
	PipelineNumber uint64    `json:"pipeline_number"`
	CreatedAt      time.Time `json:"created_at"`
	StoppedAt      time.Time `json:"stopped_at"`
}

type WorkflowJob struct {
	CanceledBy        string    `json:"canceled_by"`
	Dependencies      []string  `json:"dependencies"`
	JobNumber         int       `json:"job_number"`
	ID                string    `json:"id"`
	StartedAt         time.Time `json:"started_at"`
	Name              string    `json:"name"`
	ApprovedBy        string    `json:"approved_by"`
	ProjectSlug       string    `json:"project_slug"`
	Status            string    `json:"status"`
	Type              string    `json:"type"`
	StoppedAt         time.Time `json:"stopped_at"`
	ApprovalRequestID string    `json:"approval_request_id"`
}

func (c *Client) GetWorkflow(id string) (*Workflow, error) {
	// https://circleci.com/docs/api/v2/index.html#operation/getWorkflowById

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/workflow/%s", id))
	if err != nil {
		return nil, err
	}

	return getOne[Workflow](c, u)
}

func (c *Client) GetWorkflowJobs(id string, max int) ([]WorkflowJob, error) {
	// https://circleci.com/docs/api/v2/index.html#operation/listWorkflowJobs

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/workflow/%s/job", id))
	if err != nil {
		return nil, err
	}

	return get[WorkflowJob](c, u, max)
}
