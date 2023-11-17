package condition

import (
	"errors"

	"github.com/antonmedv/expr"
)

// ErrConditionOutputInvalid ...
// condition expresion must return boolean value
var ErrConditionOutputInvalid = errors.New("condition output invalid")

// Compute ...
func Compute(c string, render interface{}) (bool, error) {
	program, err := expr.Compile(c)
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, render)
	if err != nil {
		return false, err
	}

	// condition expresion must return boolean value
	var pass, ok bool
	if pass, ok = output.(bool); !ok {
		return false, ErrConditionOutputInvalid
	}

	return pass, nil
}
