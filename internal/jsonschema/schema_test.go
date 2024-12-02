// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.package schema

package jsonschema

import (
	"encoding/json"
	"strings"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

// these three example schemas must remain identical for tests to be correct.

const jsonExample = `
{
  "type": "object",
  "properties": {
    "payload": {
      "type": "string",
      "minLength": 5,
      "maxLength": 10,
      "secret": true,
	  "singular": "payload",
	  "plural": "payloads"
    }
  },
  "immutable": true
}
`

const yamlExample = `
type: object
properties:
  payload:
    type: string
    minLength: 5
    maxLength: 10
    secret: true
    singular: payload
    plural: payloads
immutable: true
`

var objExample = &Schema{
	Type: []Type{ObjectType},
	Properties: map[string]*Schema{
		"payload": {
			Type:      []Type{StringType},
			MinLength: Int(5),
			MaxLength: Int(10),
			Secret:    true,
			Singular:  "payload",
			Plural:    "payloads",
		},
	},
	Immutable: true,
}

type Suite struct{}

var _ = gc.Suite(Suite{})

func (Suite) TestJSONMarshal(c *gc.C) {
	s := &Schema{}
	err := json.Unmarshal([]byte(jsonExample), s)
	c.Assert(err, gc.IsNil)

	c.Check(s, gc.DeepEquals, objExample)
}

func (Suite) TestJSONRoundTrip(c *gc.C) {
	s := &Schema{}
	err := json.Unmarshal([]byte(jsonExample), s)
	c.Assert(err, gc.IsNil)

	// Note: we don't actually check that the bytes that are output are the same
	// as the original json. This is because the output doesn't omit some empty
	// values etc.  But we can re-marshal and make sure the in-memory
	// representation is the same.

	b, err := json.Marshal(s)
	c.Assert(err, gc.IsNil)
	s2 := &Schema{}
	err = json.Unmarshal(b, s2)
	c.Assert(err, gc.IsNil)
	c.Check(s, gc.DeepEquals, s2)
}

func (Suite) TestFromJSON(c *gc.C) {
	s, err := FromJSON(strings.NewReader(jsonExample))
	c.Assert(err, gc.IsNil)

	c.Check(s, gc.DeepEquals, objExample)
}

func (Suite) TestFromYAML(c *gc.C) {
	s, err := FromYAML(strings.NewReader(yamlExample))
	c.Assert(err, gc.IsNil)

	c.Check(s, jc.DeepEquals, objExample)
}

func (Suite) TestValidateMaps(c *gc.C) {
	err := objExample.Validate(map[string]interface{}{"payload": "123456"})
	c.Check(err, gc.IsNil)

	// string is too short, should fail.
	err = objExample.Validate(map[string]interface{}{"payload": "123"})
	c.Check(err, gc.NotNil)
}

func (Suite) TestValidateNonMap(c *gc.C) {
	s := &Schema{
		Type:      []Type{StringType},
		MinLength: Int(5),
		MaxLength: Int(10),
	}

	err := s.Validate("123456")
	c.Check(err, gc.IsNil)

	// string is too short, should fail.
	err = s.Validate("123")
	c.Check(err, gc.NotNil)

	s = &Schema{
		Type: []Type{ArrayType},
		Items: &ItemSpec{
			Schemas: []*Schema{{
				Type: []Type{IntegerType},
			}},
		},
	}

	// slice of interface values with correct wrapped value ok.
	err = s.Validate([]interface{}{5, 10, 20})
	c.Check(err, gc.IsNil)

	// direct slice of expected types also ok.
	err = s.Validate([]int{5, 10, 20})
	c.Check(err, gc.IsNil)
}

func (Suite) TestInsertDefaults(c *gc.C) {
	s := &Schema{
		Type: []Type{ObjectType},
		Properties: map[string]*Schema{
			"payload": &Schema{
				Type:      []Type{StringType},
				MinLength: Int(5),
				MaxLength: Int(10),
				Default:   "XXXXX",
			},
			"size": &Schema{
				Type:    []Type{IntegerType},
				Default: 5,
			},
			"data": &Schema{
				Type: []Type{ObjectType},
				Properties: map[string]*Schema{
					"isFoo": &Schema{
						Type:    []Type{BooleanType},
						Default: true,
					},
				},
			},
		},
	}

	m := map[string]interface{}{}
	s.InsertDefaults(m)
	// empty -> filled
	c.Check(m, gc.DeepEquals, map[string]interface{}{
		"payload": "XXXXX",
		"size":    5,
		"data": map[string]interface{}{
			"isFoo": true,
		},
	})

	m = map[string]interface{}{
		"payload": "YYYYY",
		"size":    10,
		"data": map[string]interface{}{
			"isFoo": false,
		},
	}
	s.InsertDefaults(m)
	// filled -> unchanged
	c.Check(m, gc.DeepEquals, map[string]interface{}{
		"payload": "YYYYY",
		"size":    10,
		"data": map[string]interface{}{
			"isFoo": false,
		},
	})

	m = map[string]interface{}{
		"size": 10,
		"data": map[string]interface{}{},
	}
	s.InsertDefaults(m)
	// partially filled -> filled (including adding missing sub properties)
	c.Check(m, gc.DeepEquals, map[string]interface{}{
		"payload": "XXXXX",
		"size":    10,
		"data": map[string]interface{}{
			"isFoo": true,
		},
	})
}
