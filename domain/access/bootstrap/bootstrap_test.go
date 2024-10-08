// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package bootstrap

import (
	"context"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/permission"
	usertesting "github.com/juju/juju/core/user/testing"
	schematesting "github.com/juju/juju/domain/schema/testing"
	"github.com/juju/juju/internal/auth"
)

type bootstrapSuite struct {
	schematesting.ControllerSuite

	controllerUUID string
}

var _ = gc.Suite(&bootstrapSuite{})

func (s *bootstrapSuite) SetUpTest(c *gc.C) {
	s.ControllerSuite.SetUpTest(c)
	s.controllerUUID = s.SeedControllerUUID(c)
}

func (s *bootstrapSuite) TestAddUserWithPassword(c *gc.C) {
	ctx := context.Background()
	uuid, addAdminUser := AddUserWithPassword(usertesting.GenNewName(c, "admin"), auth.NewPassword("password"), permission.AccessSpec{
		Access: permission.SuperuserAccess,
		Target: permission.ID{
			ObjectType: permission.Controller,
			Key:        s.controllerUUID,
		},
	})
	err := addAdminUser(ctx, s.TxnRunner(), s.NoopTxnRunner())
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(uuid.Validate(), jc.ErrorIsNil)

	// Check that the user was created.
	var name string
	row := s.DB().QueryRow(`
SELECT name FROM user WHERE name = ?`, "admin")
	c.Assert(row.Scan(&name), jc.ErrorIsNil)
	c.Assert(name, gc.Equals, "admin")
}
