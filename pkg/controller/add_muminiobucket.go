package controller

import (
	"github.com/cbenien/muminio/pkg/controller/muminiobucket"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, muminiobucket.Add)
}
