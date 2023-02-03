package main

import (
	"fmt"
	"net/url"
	"time"
)

type Job struct {
	WebURL         string         `json:"web_url"`
	Project        Project        `json:"project"`
	ParallelRuns   []ParallelRuns `json:"parallel_runs"`
	StartedAt      time.Time      `json:"started_at"`
	LatestWorkflow LatestWorkflow `json:"latest_workflow"`
	Name           string         `json:"name"`
	Executor       Executor       `json:"executor"`
	Parallelism    int            `json:"parallelism"`
	Status         string         `json:"status"`
	Number         int            `json:"number"`
	Pipeline       Pipeline       `json:"pipeline"`
	Duration       int            `json:"duration"`
	CreatedAt      time.Time      `json:"created_at"`
	Messages       []Messages     `json:"messages"`
	Contexts       []Contexts     `json:"contexts"`
	Organization   Organization   `json:"organization"`
	QueuedAt       time.Time      `json:"queued_at"`
	StoppedAt      time.Time      `json:"stopped_at"`
}

type ParallelRuns struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
}

type LatestWorkflow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Executor struct {
	ResourceClass string `json:"resource_class"`
	Type          string `json:"type"`
}

type Messages struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

type Contexts struct {
	Name string `json:"name"`
}

type Organization struct {
	Name string `json:"name"`
}

func (c *Client) GetJob(project, jobNumber int) (*Job, error) {
	// https://circleci.com/docs/api/v2/index.html#tag/Context

	u, err := url.Parse(fmt.Sprintf("https://circleci.com/api/v2/project/%s/%s/job/%i", c.orgSlug, project, jobNumber))
	if err != nil {
		return nil, err
	}

	return getOne[Job](c, u)
}
