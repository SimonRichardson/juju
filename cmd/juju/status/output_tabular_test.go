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
					"mediawiki/0": {Leader: true, Branch: "#1"},
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
	c.Check(strings.Contains(output, "mediawiki/0* #1"), tc.IsTrue)
	c.Check(strings.Contains(output, "mediawiki/1"), tc.IsTrue)
}
