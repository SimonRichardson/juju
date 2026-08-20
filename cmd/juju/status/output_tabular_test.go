// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package status

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/juju/tc"

	"github.com/juju/juju/cmd/cmd"
	jujutesting "github.com/juju/juju/internal/testing"
	"github.com/juju/juju/rpc/params"
)

func TestOutputTabularSuite(t *testing.T) {
	tc.Run(t, &outputTabularSuite{})
}

type outputTabularSuite struct {
	jujutesting.BaseSuite
}

func (s *outputTabularSuite) TestFormatOnelinePortsGroupedNumerically(c *tc.C) {
	fs := formattedStatus{
		Applications: map[string]applicationStatus{
			"app": {
				Units: map[string]unitStatus{
					"app/0": {
						OpenedPorts: []string{
							"9998/tcp",
							"9999/tcp",
							"10000/tcp",
							"10002/tcp",
							"10003/tcp",
							"10004/tcp",
						},
					},
				},
			},
		},
	}

	buff := &bytes.Buffer{}
	err := formatOneline(buff, false, fs, func(out io.Writer, format, uName string, u unitStatus, level int) {
		fmt.Fprintf(out, format, uName, "running", level)
	})
	c.Assert(err, tc.ErrorIsNil)

	c.Assert(buff.String(), tc.Contains, "9998-10000,10002-10004/tcp")
}

func (s *outputTabularSuite) TestFormatterAddsBranchesAndUnitReferences(c *tc.C) {
	status := &params.FullStatus{
		Model: params.ModelStatusInfo{CloudTag: "cloud-dummy"},
		Branches: map[string]params.BranchStatus{
			"test": {
				AssignedUnits: map[string][]string{"mediawiki": {"mediawiki/0"}},
				Created:       time.Now().Add(-9*time.Minute - 10*time.Second).Unix(),
				CreatedBy:     "admin",
			},
		},
		Applications: map[string]params.ApplicationStatus{
			"mediawiki": {
				Charm: "mediawiki",
				Units: map[string]params.UnitStatus{
					"mediawiki/0": {Leader: true},
					"mediawiki/1": {},
				},
			},
		},
	}
	formatter := NewStatusFormatter(NewStatusFormatterParams{
		Status:     status,
		OutputName: "tabular",
	})

	formatted, err := formatter.Format()
	c.Assert(err, tc.ErrorIsNil)
	c.Check(formatted.Branches, tc.DeepEquals, map[string]branchStatus{
		"test": {
			Ref:       "#1",
			Created:   "9 minutes ago",
			CreatedBy: "admin",
			Active:    true,
		},
	})
	c.Check(formatted.Applications["mediawiki"].Units["mediawiki/0"].Branch, tc.Equals, "#1")
	c.Check(formatted.Applications["mediawiki"].Units["mediawiki/1"].Branch, tc.Equals, "")
}

func (s *outputTabularSuite) TestFormatTabularBranches(c *tc.C) {
	revision := 23
	formatted := formattedStatus{
		Model: modelStatus{Name: "default", Type: "iaas"},
		Branches: map[string]branchStatus{
			"test": {
				Ref:       "#1",
				Created:   "9 minutes ago",
				CreatedBy: "admin",
				Active:    true,
			},
		},
		Applications: map[string]applicationStatus{
			"mediawiki": {
				Units: map[string]unitStatus{
					"mediawiki/0": {
						Leader: true, Branch: "#1", Charm: "ch:amd64/mediawiki-23", CharmRev: &revision,
					},
					"mediawiki/1": {},
				},
			},
		},
		Machines: map[string]machineStatus{},
	}

	buff := &bytes.Buffer{}
	err := FormatTabular(buff, false, formatted)
	c.Assert(err, tc.ErrorIsNil)
	output := buff.String()
	c.Check(strings.Contains(output, "Branch  Ref  Created"), tc.IsTrue)
	c.Check(strings.Contains(output, "test*   #1   9 minutes ago  admin"), tc.IsTrue)
	c.Check(statusLineFields(output, "Unit"), tc.DeepEquals,
		[]string{"Unit", "Charm", "Workload", "Agent", "Machine", "Public", "address", "Ports", "Message"})
	c.Check(statusLineFields(output, "mediawiki/0*"), tc.DeepEquals,
		[]string{"mediawiki/0*", "#1", "23"})
	c.Check(statusLineFields(output, "mediawiki/1"), tc.DeepEquals,
		[]string{"mediawiki/1", "-"})
}

func (s *outputTabularSuite) TestFormatTabularHidesUnitCharmWithoutBranch(c *tc.C) {
	formatted := formattedStatus{
		Model: modelStatus{Name: "default", Type: "iaas"},
		Applications: map[string]applicationStatus{
			"mediawiki": {
				Units: map[string]unitStatus{
					"mediawiki/0": {Charm: "ch:amd64/mediawiki-23"},
				},
			},
		},
		Machines: map[string]machineStatus{},
	}

	buff := &bytes.Buffer{}
	err := FormatTabular(buff, false, formatted)
	c.Assert(err, tc.ErrorIsNil)
	output := buff.String()
	c.Check(statusLineFields(output, "Unit"), tc.DeepEquals,
		[]string{"Unit", "Workload", "Agent", "Machine", "Public", "address", "Ports", "Message"})
}

func statusLineFields(output, prefix string) []string {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == prefix {
			if prefix != "Unit" && len(fields) > 3 {
				return fields[:3]
			}
			return fields
		}
	}
	return nil
}

func (s *outputTabularSuite) TestStructuredOutputIncludesDivergentUnitCharm(c *tc.C) {
	status := &params.FullStatus{
		Model: params.ModelStatusInfo{CloudTag: "cloud-dummy"},
		Branches: map[string]params.BranchStatus{
			"test": {},
		},
		Applications: map[string]params.ApplicationStatus{
			"mediawiki": {
				Charm: "ch:amd64/mediawiki-21",
				Units: map[string]params.UnitStatus{
					"mediawiki/0": {Charm: "ch:amd64/mediawiki-23"},
					"mediawiki/1": {},
				},
			},
		},
	}
	formatter := NewStatusFormatter(NewStatusFormatterParams{Status: status, OutputName: "yaml"})
	formatted, err := formatter.Format()
	c.Assert(err, tc.ErrorIsNil)

	diverged := formatted.Applications["mediawiki"].Units["mediawiki/0"]
	c.Check(diverged.CharmURL, tc.Equals, "ch:amd64/mediawiki-23")
	c.Assert(diverged.CharmRev, tc.NotNil)
	c.Check(*diverged.CharmRev, tc.Equals, 23)
	matching := formatted.Applications["mediawiki"].Units["mediawiki/1"]
	c.Check(matching.CharmURL, tc.Equals, "")
	c.Check(matching.CharmRev, tc.IsNil)

	yamlOutput := &bytes.Buffer{}
	c.Assert(cmd.FormatYaml(yamlOutput, formatted), tc.ErrorIsNil)
	c.Check(yamlOutput.String(), tc.Contains, "charm-url: ch:amd64/mediawiki-23")
	c.Check(yamlOutput.String(), tc.Contains, "charm-rev: 23")

	jsonOutput := &bytes.Buffer{}
	c.Assert(cmd.FormatJson(jsonOutput, formatted), tc.ErrorIsNil)
	c.Check(jsonOutput.String(), tc.Contains, `"charm-url":"ch:amd64/mediawiki-23"`)
	c.Check(jsonOutput.String(), tc.Contains, `"charm-rev":23`)
}
