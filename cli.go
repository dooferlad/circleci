package main

import (
	"fmt"
	"github.com/fatih/color"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type PipelineState struct {
	gorm.Model
	ID     string `gorm:"primaryKey"`
	State  string
	Result string
}

type JobState struct {
	gorm.Model
	ID         string `gorm:"primaryKey"`
	PipelineID string
	State      string
	Result     string
	URL        string
	Name       string
}

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Fatal("Error loading .env file")
	}

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		logrus.Fatal(err)
	}

	// Migrate the schema
	if err := db.AutoMigrate(&PipelineState{}); err != nil {
		logrus.Fatal("Error migrating PipelineState", err)
	}
	if err := db.AutoMigrate(&JobState{}); err != nil {
		logrus.Fatal("Error migrating JobState", err)
	}

	c := NewClient(os.Getenv("CIRCLECI_TOKEN"), os.Getenv("CIRCLECI_ORG_SLUG"))

	var pipelineName string
	if len(os.Args) > 1 {
		pipelineName = os.Args[1]
	}

	v, err := c.GetProjectPipelines(os.Getenv("CIRCLECI_PROJECT"), os.Getenv("CIRCLECI_BRANCH"), 5)
	if err != nil {
		logrus.Fatal(err)
	}

	for _, i := range v {
		var subject string
		var getDetail bool
		if i.Vcs.Commit.Subject == "" {
			subject = i.Vcs.Commit.Body
		} else {
			subject = i.Vcs.Commit.Subject
		}

		// If we have got a pipelineName to look for, don't print other info
		if pipelineName != "" {
			if !strings.Contains(subject, pipelineName) {
				continue
			}
			getDetail = true
		}

		pipelineState := &PipelineState{}
		result := db.Limit(1).Find(&pipelineState, "id = ?", i.ID)
		if result.RowsAffected == 0 {
			pipelineState.ID = i.ID
			db.Create(&pipelineState)
		}

		if pipelineState.State != "complete" {
			getPipelineState(c, i, pipelineState, db, getDetail)
		}

		db.Limit(1).Find(&pipelineState, "id = ?", i.ID)

		var stateString string
		if pipelineState.State == "complete" {
			if pipelineState.Result == "failed" {
				color.Set(color.FgRed)
				stateString = "✗"
			} else {
				color.Set(color.FgGreen)
				stateString = "✓"
			}
		} else {
			if pipelineState.State == "running" {
				color.Set(color.FgBlue)
				stateString = ">"
			} else {
				stateString = " "
			}
		}

		fmt.Println(stateString, subject)
		color.Set(color.Reset)

		if pipelineState.Result == "failed" || getDetail {
			var jobs []JobState
			db.Where("pipeline_id = ?", pipelineState.ID).Find(&jobs)

			for _, job := range jobs {
				if (getDetail && job.Result != "success") || (!getDetail && job.Result == "failed") {
					switch job.Result {
					case "running":
						color.Set(color.FgHiBlue)
					case "blocked":
						color.Set(color.FgYellow)
					case "failed":
						color.Set(color.FgRed)
					}
					fmt.Printf("   %60s %10s %s\n", job.Name, job.Result, job.URL)
					color.Set(color.Reset)
				}
			}
		}
	}
	return
}

func getPipelineState(c *Client, i Pipeline, pipelineState *PipelineState, db *gorm.DB, force bool) {
	org := os.Getenv("CIRCLECI_ORG")
	project := os.Getenv("CIRCLECI_PROJECT")

	if pipelineWorkflows, err := c.GetPipelineWorkflows(i.ID, 10); err != nil {
		logrus.Fatal(err)
	} else {
		failed := false
		running := false

		for _, workflow := range pipelineWorkflows {
			if workflow.Status == "failed" {
				failed = true
			} else if workflow.Status != "success" {
				running = true
			}

			if workflow.Status == "failed" || force {
				if job, err := c.GetWorkflowJobs(workflow.ID, 100); err != nil {
					logrus.Fatal(err)
				} else {
					for _, i := range job {
						url := fmt.Sprintf(
							"https://app.circleci.com/pipelines/github/%s/%s/%d/workflows/%s/jobs/%d",
							org, project, workflow.PipelineNumber, workflow.ID, i.JobNumber,
						)
						jobState := &JobState{}

						result := db.Limit(1).Find(&jobState, "id = ?", i.ID)

						jobState.ID = i.ID
						jobState.URL = url
						jobState.Result = i.Status
						jobState.PipelineID = pipelineState.ID
						jobState.Name = i.Name

						if result.RowsAffected == 0 {
							db.Create(&jobState)
						} else {
							db.Updates(&jobState)
						}
					}
				}
			}
		}

		if failed {
			pipelineState.Result = "failed"

		} else if !running {
			pipelineState.Result = "pass"
		}

		if running {
			pipelineState.State = "running"
		} else {
			pipelineState.State = "complete"
		}

		db.Updates(&pipelineState)
	}
}
