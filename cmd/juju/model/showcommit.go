// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"strconv"
	"time"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/core/output"
	"github.com/juju/juju/rpc/params"
)

// NewShowCommitCommand returns the command for showing a generation commit.
func NewShowCommitCommand() cmd.Command {
	return modelcmd.Wrap(&showCommitCommand{})
}

type showCommitCommand struct {
	generationCommandBase
	out          cmd.Output
	generationID int
	isoTime      bool
}

func (c *showCommitCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "show-commit",
		Args:    "<generation id>",
		Purpose: "Shows details of a committed model branch.",
		Examples: `
    juju show-commit 3
    juju show-commit 3 --format json --utc
`,
		SeeAlso: []string{"commits", "commit"},
	})
}

func (c *showCommitCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
	f.BoolVar(&c.isoTime, "utc", false, "Display times in UTC")
	c.out.AddFlags(f, "yaml", output.DefaultFormatters)
}

func (c *showCommitCommand) Init(args []string) error {
	if len(args) != 1 {
		return errors.Errorf("expected exactly one generation id, got %d arguments", len(args))
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return errors.Errorf("invalid generation id %q", args[0])
	}
	if id < 0 {
		return errors.New("generation id cannot be negative")
	}
	c.generationID = id
	return initISOTimestamp(&c.isoTime)
}

func (c *showCommitCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	commit, err := client.ShowCommit(ctx, c.generationID)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(c.out.Write(ctx, c.formatCommit(commit)))
}

func (c *showCommitCommand) formatCommit(commit params.Generation) formattedShowCommit {
	completed := time.Unix(commit.Completed, 0)
	result := formattedShowCommit{
		GenerationID: commit.GenerationId,
		Branch: map[string]formattedBranchCommit{
			commit.BranchName: {Applications: formatCommitApplications(commit.Applications)},
		},
		CommittedAt: common.FormatTime(&completed, c.isoTime),
		CommittedBy: commit.CompletedBy,
		CreatedBy:   commit.CreatedBy,
	}
	if commit.Created != 0 {
		created := time.Unix(commit.Created, 0)
		formatted := common.FormatTime(&created, c.isoTime)
		result.Created = &formatted
	}
	return result
}

func formatCommitApplications(applications []params.GenerationApplication) []formattedCommitApplication {
	result := make([]formattedCommitApplication, len(applications))
	for i, application := range applications {
		result[i] = formattedCommitApplication{
			ApplicationName: application.ApplicationName,
			ConfigChanges:   application.ConfigChanges,
		}
	}
	return result
}

type formattedShowCommit struct {
	GenerationID int                              `json:"generation-id" yaml:"generation-id"`
	Branch       map[string]formattedBranchCommit `json:"branch" yaml:"branch"`
	CommittedAt  string                           `json:"committed-at" yaml:"committed-at"`
	CommittedBy  string                           `json:"committed-by" yaml:"committed-by"`
	Created      *string                          `json:"created,omitempty" yaml:"created,omitempty"`
	CreatedBy    string                           `json:"created-by" yaml:"created-by"`
}

type formattedBranchCommit struct {
	Applications []formattedCommitApplication `json:"applications" yaml:"applications"`
}

type formattedCommitApplication struct {
	ApplicationName string         `json:"application" yaml:"application"`
	ConfigChanges   map[string]any `json:"config" yaml:"config"`
}
