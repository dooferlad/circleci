package main

import (
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	flag "github.com/spf13/pflag"
	"os"
	"slices"
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
	Number            int64
	State             string
	Result            string
	Subject           string
	Branch            string
	User              string
	Revision          string
	PipelineUpdatedAt time.Time
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
	flag.IntVarP(&maxPipelines, "max-pipelines", "x", 30, "Maximum pipelines to fetch")

	var pipelineName string
	flag.StringVarP(&pipelineName, "pipeline-name", "n", "", "Pipeline name to filter results on")

	var skipFetch bool
	flag.BoolVarP(&skipFetch, "skip-pipeline-fetch", "s", false, "Skip fetching pipeline list from API")

	var printPipelines bool
	flag.BoolVarP(&printPipelines, "print-all", "p", false, "Print all jobs in a pipeline, not just failing ones")

	var branchName string
	flag.StringVarP(&branchName, "branch-name", "b", os.Getenv("CIRCLECI_BRANCH"), "Branch to examine pipelines for")

	var projectName string
	flag.StringVarP(&projectName, "project-name", "r", os.Getenv("CIRCLECI_PROJECT"), "Project to examine pipelines for")

	var filterRevision string
	flag.StringVarP(&filterRevision, "revision", "v", "", "VCS revision to filter on")

	var filterUser string
	flag.StringVarP(&filterUser, "username", "u", "", "VCS user to filter on")

	var defaultGetDetail bool
	flag.BoolVarP(&defaultGetDetail, "get-detail", "a", false, "Get all job detail")

	var forceFetch bool
	flag.BoolVarP(&forceFetch, "force-fetch", "f", false, "Force fetching from CircleCI")

	var pipelineNumber int
	flag.IntVarP(&pipelineNumber, "pipeline-number", "i", 0, "Maximum pipelines to fetch")

	var getResourceUsage bool
	flag.BoolVarP(&getResourceUsage, "get-resource-usage", "g", false, "Get resource usage (currently RAM) for jobs")

	flag.Parse()

	// Migrate the schema
	if err := db.AutoMigrate(&PipelineState{}); err != nil {
		logrus.Fatal("Error migrating PipelineState: ", err)
	}
	if err := db.AutoMigrate(&JobState{}); err != nil {
		logrus.Fatal("Error migrating JobState: ", err)
	}

	c := NewClient(os.Getenv("CIRCLECI_TOKEN"), os.Getenv("CIRCLECI_ORG_SLUG"))

	var pipelineID string

	if !skipFetch {
		var v []Pipeline
		var err error
		if pipelineNumber > 0 {
			p, err := c.GetPipelineByNumber(projectName, pipelineNumber)
			if err != nil {
				logrus.Fatal("GetProjectPipelines: ", err)
			}
			v = append(v, *p)
			logrus.Info(fmt.Sprintf("%#v", p))
			filterRevision = v[0].Vcs.Revision
			pipelineID = v[0].ID
		} else {
			logrus.Infof("Fetching pipelines for project %s, branch %s", projectName, branchName)
			v, err = c.GetProjectPipelines(projectName, branchName, maxPipelines)
		}
		if err != nil {
			logrus.Fatal("GetProjectPipelines: ", err)
		}

		for _, i := range v {
			pipelineState := &PipelineState{}

			result := db.Limit(1).Find(&pipelineState, "id = ?", i.ID)
			if result.RowsAffected == 0 {
				pipelineState.ID = i.ID
				pipelineState.Branch = branchName
				pipelineState.Number = i.Number
				pipelineState.User = i.Trigger.Actor.Login
				pipelineState.Revision = i.Vcs.Revision

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
	var query *gorm.DB

	if pipelineID != "" {
		query = db.Limit(1).Where("id = ?", pipelineID)
	} else {
		query = db.Limit(maxPipelines).Where("branch = ?", branchName)
		if pipelineName != "" {
			query = query.Where("subject LIKE ?", "%"+pipelineName+"%")
		}
		if filterRevision != "" {
			query = query.Where("revision = ?", filterRevision)
		}
		if filterUser != "" {
			query = query.Where("user = ?", filterUser)
		}
	}

	query.Order("-number").FindInBatches(&pipelineStates, 10, func(tx *gorm.DB, batch int) error {
		for _, pipelineState := range pipelineStates {
			getDetail := defaultGetDetail

			// If we have got a pipelineName to look for, don't print other info
			if pipelineName != "" {
				if !strings.Contains(pipelineState.Subject, pipelineName) {
					continue
				}
				getDetail = true
			}

			if pipelineState.State != "complete" && !skipFetch || forceFetch {
				getPipelineState(c, pipelineState.ID, pipelineState, db, forceFetch, getResourceUsage)
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

			fmt.Println(stateString, pipelineState.Number, pipelineState.Subject, pipelineState.PipelineUpdatedAt, pipelineState.User, pipelineState.Revision)

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

type PipelineStats struct {
	Name     string
	MaxRAM   int
	MaxCPU   float32
	Executor string
}

func getPipelineState(c *Client, id string, pipelineState *PipelineState, db *gorm.DB, force, getResourceUsage bool) {
	org := os.Getenv("CIRCLECI_ORG")
	project := os.Getenv("CIRCLECI_PROJECT")
	var pipelineStats []PipelineStats

	if pipelineWorkflows, err := c.GetPipelineWorkflows(id, 10); err != nil {
		logrus.Error("getPipelineState -> GetPipelineWorkflows: ", id, " ", err)
		return
	} else {
		failed := false
		running := false
		numFetchTokens := 25

		tokenBucket := make(chan struct{}, numFetchTokens)
		for i := 0; i < numFetchTokens; i++ {
			tokenBucket <- struct{}{}
		}
		results := make(chan PipelineStats, 10)
		gotAllResults := make(chan struct{})
		expectedResults := 0

		go func() {
			logrus.Infof("Listening for results of %d workflows", len(pipelineWorkflows))
			receivedResults := 0

			for receivedResults < expectedResults || expectedResults == 0 || len(tokenBucket) < numFetchTokens {
				select {
				case result, ok := <-results:
					if !ok {
						break
					}
					logrus.Infof("%d %d/%d: %s: Max RAM: %d, %s", len(tokenBucket), receivedResults, expectedResults, result.Name, result.MaxRAM, result.Executor)
					pipelineStats = append(pipelineStats, result)
					receivedResults++
				case <-time.After(time.Second):
					logrus.Infof("%d < %d || %d == 0 || %d < %d", receivedResults, expectedResults, expectedResults, len(tokenBucket), numFetchTokens)
				}
			}

			logrus.Info("Got all results")
			close(gotAllResults)
		}()

		for _, workflow := range pipelineWorkflows {
			go func(workflow Workflow) {
				if workflow.Status == "failed" {
					failed = true
				} else if workflow.Status != "success" {
					running = true
				}

				if workflow.Status == "failed" || force {
					if jobs, err := c.GetWorkflowJobs(workflow.ID, 100); err != nil {
						logrus.Fatal("getPipelineState -> GetWorkflowJobs: ", err)
					} else {
						expectedResults += len(jobs)

						for _, job := range jobs {
							go func(job WorkflowJob) {
								url := fmt.Sprintf(
									"https://app.circleci.com/pipelines/github/%s/%s/%d/workflows/%s/jobs/%d",
									org, project, workflow.PipelineNumber, workflow.ID, job.JobNumber,
								)
								jobState := &JobState{}

								result := db.Limit(1).Find(&jobState, "id = ?", job.ID)

								jobState.ID = job.ID
								jobState.URL = url
								jobState.Result = job.Status
								jobState.PipelineID = pipelineState.ID
								jobState.Name = job.Name

								if getResourceUsage {
									<-tokenBucket                                // Grab a token from the bucket
									defer func() { tokenBucket <- struct{}{} }() // return token to the bucket

									jobDetail, err := c.GetJob(job.ProjectSlug, job.JobNumber)
									if err != nil {
										logrus.Error(err)
										return
									}

									stats, err := c.GetJobUsage(&job)
									if err != nil {
										logrus.Error(err)
									} else {
										var maxRAM float32
										var maxCPU float32
										for _, usage := range *stats {
											if m := slices.Max(usage.MemoryBytes); m > maxRAM {
												maxRAM = m
											}

											if m := slices.Max(usage.CPU); m > maxCPU {
												maxCPU = m
											}
										}
										// logrus.Infof("%s: Max RAM: %f, %#v", job.Name, outerMax/(1024.0*1024.0), jobDetail.Executor.ResourceClass)
										results <- PipelineStats{Name: job.Name, MaxRAM: int(maxRAM), MaxCPU: maxCPU, Executor: jobDetail.Executor.ResourceClass}
										// pipelineStats[job.Name] = outerMax
									}
								}

								if result.RowsAffected == 0 {
									db.Create(&jobState)
								} else {
									db.Updates(&jobState)
								}
							}(job)
						}
					}

				}
			}(workflow)
		}

		<-gotAllResults // Block until we have all the results

		if getResourceUsage {
			if f, err := os.Create(fmt.Sprintf("%s.json", id)); err != nil {
				logrus.Error(err)
			} else {
				defer f.Close()
				json.NewEncoder(f).Encode(pipelineStats)
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
