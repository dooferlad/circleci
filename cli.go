package main

import (
	"flag"
	"fmt"
	"github.com/fatih/color"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type PipelineState struct {
	gorm.Model
	ID                string `gorm:"primaryKey"`
	State             string
	Result            string
	Subject           string
	PipelineUpdatedAt time.Time
	UpdatedAt         time.Time
}

type JobState struct {
	gorm.Model
	ID         string `gorm:"primaryKey"`
	PipelineID string `gorm:"index"`
	State      string
	Result     string
	URL        string
	Name       string
}

func dbMust(db *gorm.DB, sql string) {
	if res := db.Exec(sql); res.Error != nil {
		logrus.Fatal(res.Error)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Fatal("Error loading .env file")
	}

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		logrus.Fatal("gorm.Open: ", err)
	}

	dbMust(db, "PRAGMA synchronous = NORMAL")
	dbMust(db, "pragma vacuum")
	dbMust(db, "pragma optimize")
	dbMust(db, "pragma journal_mode = WAL")
	dbMust(db, "pragma temp_store = memory")
	dbMust(db, "pragma mmap_size = 30000000000")
	dbMust(db, "pragma page_size = 32768")

	var maxPipelines int
	flag.IntVar(&maxPipelines, "x", 5, "Maximum pipelines to fetch")
	flag.IntVar(&maxPipelines, "max-pipelines", 5, "Maximum pipelines to fetch")

	var pipelineName string
	flag.StringVar(&pipelineName, "n", "", "Pipeline name to filter results on")
	flag.StringVar(&pipelineName, "pipeline-name", "", "Pipeline name to filter results on")

	var skipFetch bool
	flag.BoolVar(&skipFetch, "s", false, "Skip fetching pipeline list from API")
	flag.BoolVar(&skipFetch, "skip-pipeline-fetch", false, "Skip fetching pipeline list from API")

	var printPipelines bool
	flag.BoolVar(&printPipelines, "p", false, "Print all jobs in a pipeline, not just failing ones")
	flag.BoolVar(&printPipelines, "print-all", false, "Print all jobs in a pipeline, not just failing ones")
	flag.Parse()

	// Migrate the schema
	if err := db.AutoMigrate(&PipelineState{}); err != nil {
		logrus.Fatal("Error migrating PipelineState: ", err)
	}
	if err := db.AutoMigrate(&JobState{}); err != nil {
		logrus.Fatal("Error migrating JobState: ", err)
	}

	c := NewClient(os.Getenv("CIRCLECI_TOKEN"), os.Getenv("CIRCLECI_ORG_SLUG"))

	if !skipFetch {
		v, err := c.GetProjectPipelines(os.Getenv("CIRCLECI_PROJECT"), os.Getenv("CIRCLECI_BRANCH"), maxPipelines)
		if err != nil {
			logrus.Fatal("GetProjectPipelines: ", err)
		}

		for _, i := range v {
			pipelineState := &PipelineState{}

			result := db.Limit(1).Find(&pipelineState, "id = ?", i.ID)
			if result.RowsAffected == 0 {
				pipelineState.ID = i.ID

				if i.Vcs.Commit.Subject == "" {
					pipelineState.Subject = i.Vcs.Commit.Body
				} else {
					pipelineState.Subject = i.Vcs.Commit.Subject
				}

				pipelineState.PipelineUpdatedAt = i.UpdatedAt
				db.Create(pipelineState)
			} else {
				pipelineState.PipelineUpdatedAt = i.UpdatedAt
				db.Updates(pipelineState)
			}
		}
	}

	var pipelineStates []*PipelineState

	query := db.Limit(maxPipelines)
	if pipelineName != "" {
		query = query.Where("subject LIKE ?", "%"+pipelineName+"%")
	}

	query.Order("-updated_at").FindInBatches(&pipelineStates, 10, func(tx *gorm.DB, batch int) error {
		for _, pipelineState := range pipelineStates {
			var getDetail bool

			// If we have got a pipelineName to look for, don't print other info
			if pipelineName != "" {
				if !strings.Contains(pipelineState.Subject, pipelineName) {
					continue
				}
				getDetail = true
			}

			if pipelineState.State != "complete" && !skipFetch {
				getPipelineState(c, pipelineState.ID, pipelineState, db, getDetail)
			}

			numRunning := 0
			numBlocked := 0
			numPassed := 0
			numFailed := 0
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

			fmt.Println(stateString, pipelineState.Subject, pipelineState.UpdatedAt)

			if pipelineState.Result == "failed" || getDetail {
				var jobs []JobState
				db.Where("pipeline_id = ?", pipelineState.ID).Find(&jobs)

				for _, job := range jobs {
					if (getDetail && job.Result != "success") || job.Result == "failed" {
						switch job.Result {
						case "running":
							numRunning++
							if printPipelines {
								color.Set(color.FgHiBlue)
								fmt.Printf("   %60s %10s %s\n", job.Name, job.Result, job.URL)
							}
						case "blocked":
							if printPipelines {
								color.Set(color.FgYellow)
								fmt.Printf("   %60s %10s %s\n", job.Name, job.Result, job.URL)
							}
							numBlocked++
						case "failed":
							color.Set(color.FgRed)
							numFailed++
							fmt.Printf("   %60s %10s %s\n", job.Name, job.Result, job.URL)
						}
						//fmt.Printf("   %60s %10s %s\n", job.Name, job.Result, job.URL)
						color.Set(color.Reset)
					} else if job.Result == "success" {
						numPassed++
					}
				}
				fmt.Printf("  Running %d, Blocked %d, Passed %d, Failed %d\n", numRunning, numBlocked, numPassed, numFailed)
			}

			color.Set(color.Reset)
		}

		tx.Save(pipelineStates)

		return nil
	})
	return
}

func getPipelineState(c *Client, id string, pipelineState *PipelineState, db *gorm.DB, force bool) {
	org := os.Getenv("CIRCLECI_ORG")
	project := os.Getenv("CIRCLECI_PROJECT")

	if pipelineWorkflows, err := c.GetPipelineWorkflows(id, 10); err != nil {
		logrus.Error("getPipelineState -> GetPipelineWorkflows: ", id, " ", err)
		return
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
					logrus.Fatal("getPipelineState -> GetWorkflowJobs: ", err)
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
	}
}
