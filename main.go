package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/go-playground/validator"
	"github.com/google/go-github/v50/github"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ConfigFile string `validate:"required,file|url"`
	Labels     []*github.Label
	Milestones []*github.Milestone
}

func (c *Config) Validate() error {
	return validator.New().Struct(c)
}

func main() {
	action := githubactions.New()
	action.Infof("milisman starting")
	action.AddStepSummary("# Milisman Report")

	c := Config{
		ConfigFile: action.GetInput("configfile"),
	}

	if err := c.Validate(); err != nil {
		action.Fatalf("missing configfile: %s", err)
	}

	cfgfile := []byte{}

	if _, err := url.ParseRequestURI(c.ConfigFile); err != nil {
		cfgfile, err = os.ReadFile(c.ConfigFile)
		if err != nil {
			action.Fatalf("failed to read config file: %s", err)
		}
	} else {
		resp, err := http.Get(c.ConfigFile)
		if err != nil {
			action.Fatalf("failed to read config file: %s", err)
		}
		defer resp.Body.Close()
		cfgfile, err = io.ReadAll(resp.Body)
	}

	err := yaml.Unmarshal(cfgfile, &c)
	if err != nil {
		action.Fatalf("failed to parse config file: %s", err)
	}

	if err := c.Validate(); err != nil {
		action.Fatalf("invalid configuration: %s", err)
	}
	action.AddStepSummary("Using config file: " + c.ConfigFile)

	ghcontext, err := githubactions.Context()
	if err != nil {
		action.Fatalf("failed to get github context: %s", err)
	}
	ghowner, ghname := ghcontext.Repo()
	if len(ghowner) == 0 || len(ghname) == 0 {
		action.Fatalf("failed to get github context.")
	}

	token := action.Getenv("GITHUB_TOKEN")
	if len(token) == 0 {
		action.Fatalf("GITHUB_TOKEN is not set")
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	client := github.NewClient(httpClient)

	// create labels based on the configuration using github API
	action.Group("Labels")
	action.AddStepSummary("# Labels")
	for _, label := range c.Labels {
		_, _, err = client.Issues.CreateLabel(context.Background(), ghowner, ghname, label)
		if err != nil {
			if err.(*github.ErrorResponse).Errors[0].Code == "already_exists" {
				action.Infof("label %s already exists, skipping.", label.GetName())
				action.AddStepSummary(label.GetName() + " Skipped")
				continue
			}
			action.AddStepSummary(label.GetName() + " Failed")
			action.Fatalf("failed to create label: %s", err.Error())
		}
		action.AddStepSummary(label.GetName() + " Created")
		action.Infof("label %s created.", label.GetName())
	}
	action.EndGroup()
	// create milestones based on the configuration using github API
	action.Group("Milestones")
	action.AddStepSummary("# Milestones")
	for _, milestone := range c.Milestones {
		_, _, err = client.Issues.CreateMilestone(context.Background(), ghowner, ghname, milestone)
		if err != nil {
			if err.(*github.ErrorResponse).Errors[0].Code == "already_exists" {
				action.Infof("milestone %s already exists, skipping.", milestone.GetTitle())
				action.AddStepSummary(milestone.GetTitle() + " Skipped")
				continue
			}
			action.AddStepSummary(milestone.GetTitle() + " Failed")
			action.Fatalf("failed to create milestone: %s", err.Error())
		}
		action.AddStepSummary(milestone.GetTitle() + " Created")
		action.Infof("milestone %s created.", milestone.GetTitle())
	}
	action.EndGroup()
	action.AddStepSummary("Milisman finished.")
}
