package schema

import (
	"strings"
)

// OperationKind represents the kind of operation that is being performed.
type OperationKind string

const (
	// Table represents a table operation.
	Table OperationKind = "table"
	// View represents a view operation.
	View OperationKind = "view"
	// Index represents an index operation.
	Index OperationKind = "index"
	// Trigger represents a trigger operation.
	Trigger OperationKind = "trigger"
)

func (k OperationKind) String() string {
	return strings.ToUpper(string(k))
}

// Operation represents a database operation.
type Operation struct {
	Kind         OperationKind
	Name         string
	TriggerModes TriggerType
}

// TableOp returns a table operation.
func TableOp(name string) Operation {
	return Operation{
		Kind: Table,
		Name: name,
	}
}

// ViewOp returns a view operation.
func ViewOp(name string) Operation {
	return Operation{
		Kind: View,
		Name: name,
	}
}

// IndexOp returns a index operation.
func IndexOp(name string) Operation {
	return Operation{
		Kind: Index,
		Name: name,
	}
}

// TriggerType represents the type of trigger.
// The changes are bit flags so that they can be combined.
type TriggerType int

const (
	// Insert represents an insert trigger.
	Insert TriggerType = 1 << iota
	// Update represents an update trigger.
	Update
	// Delete represents a delete trigger.
	Delete
	// All represents any change of trigger.
	All = Insert | Update | Delete
)

// TriggerOp returns a trigger operation.
func TriggerOp(name string, modes TriggerType) Operation {
	return Operation{
		Kind:         Trigger,
		Name:         name,
		TriggerModes: modes,
	}
}
