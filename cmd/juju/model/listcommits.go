// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"

	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/cmd"
	"github.com/juju/juju/cmd/juju/common"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/core/output"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/juju/rpc/params"
)

// NewCommitsCommand returns the command for listing generation commits.
func NewCommitsCommand() cmd.Command {
	return modelcmd.Wrap(&commitsCommand{})
}

type commitsCommand struct {
	generationCommandBase
	out     cmd.Output
	isoTime bool
	now     func() time.Time
}

func (c *commitsCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:    "commits",
		Aliases: []string{"list-commits"},
		Purpose: "Lists committed model branches.",
		Examples: `
    juju commits
    juju commits --format yaml --utc
`,
		SeeAlso: []string{"show-commit", "add-branch", "commit"},
	})
}

func (c *commitsCommand) SetFlags(f *gnuflag.FlagSet) {
	c.ModelCommandBase.SetFlags(f)
	f.BoolVar(&c.isoTime, "utc", false, "Display times in UTC")
	c.out.AddFlags(f, "tabular", map[string]cmd.Formatter{
		"json":    cmd.FormatJson,
		"tabular": c.formatTabular,
		"yaml":    cmd.FormatYaml,
	})
}

func (c *commitsCommand) Init(args []string) error {
	if err := cmd.CheckEmpty(args); err != nil {
		return errors.Trace(err)
	}
	return initISOTimestamp(&c.isoTime)
}

func (c *commitsCommand) Run(ctx *cmd.Context) error {
	client, err := c.getGenerationAPI(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	defer client.Close()

	commits, err := client.ListCommits(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	if len(commits) == 0 && c.out.Name() == "tabular" {
		ctx.Infof("No commits to list")
		return nil
	}
	return errors.Trace(c.out.Write(ctx, c.formatCommits(commits)))
}

func (c *commitsCommand) formatCommits(commits []params.Generation) formattedCommitList {
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].GenerationId > commits[j].GenerationId
	})

	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	result := formattedCommitList{Commits: make([]formattedCommit, len(commits))}
	for i, commit := range commits {
		completed := time.Unix(commit.Completed, 0)
		completedAt := common.FormatTime(&completed, c.isoTime)
		if c.out.Name() == "tabular" {
			completedAt = common.UserFriendlyDuration(completed, now)
		}
		result.Commits[i] = formattedCommit{
			ID:          commit.GenerationId,
			BranchName:  commit.BranchName,
			CommittedAt: completedAt,
			CommittedBy: commit.CompletedBy,
		}
	}
	return result
}

func (c *commitsCommand) formatTabular(writer io.Writer, value any) error {
	list, ok := value.(formattedCommitList)
	if !ok {
		return errors.New("unexpected commits output value")
	}
	w := output.TabWriter(writer)
	defer w.Flush()
	fmt.Fprintln(w, "Commit\tCommitted at\tCommitted by\tBranch name")
	for _, commit := range list.Commits {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			commit.ID, commit.CommittedAt, commit.CommittedBy, commit.BranchName)
	}
	return nil
}

type formattedCommit struct {
	ID          int    `json:"id" yaml:"id"`
	BranchName  string `json:"branch-name" yaml:"branch-name"`
	CommittedAt string `json:"committed-at" yaml:"committed-at"`
	CommittedBy string `json:"committed-by" yaml:"committed-by"`
}

type formattedCommitList struct {
	Commits []formattedCommit `json:"commits" yaml:"commits"`
}

func initISOTimestamp(isoTime *bool) error {
	if *isoTime {
		return nil
	}
	value := os.Getenv(osenv.JujuStatusIsoTimeEnvKey)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return errors.Annotatef(err, "invalid %s env var, expected true|false", osenv.JujuStatusIsoTimeEnvKey)
	}
	*isoTime = parsed
	return nil
}
