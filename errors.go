package processing

import (
	"fmt"
	"strings"

	"github.com/go-ap/errors"
)

type errDuplicateKey struct {
	errors.Err
}

func isDuplicateKey(e error) bool {
	_, okp := e.(*errDuplicateKey)
	_, oks := e.(errDuplicateKey)
	return okp || oks
}

func (n errDuplicateKey) Is(e error) bool {
	return isDuplicateKey(e)
}

func wrapErr(err error, s string, args ...interface{}) errors.Err {
	return *errors.Annotatef(err, s, args...)
}

var ErrDuplicateObject = func(s string, p ...interface{}) errDuplicateKey {
	return errDuplicateKey{wrapErr(nil, fmt.Sprintf("Duplicate key: %s", s), p...)}
}

type groupError []error

func (g groupError) Error() string {
	s := strings.Builder{}
	for i, err := range g {
		if i > 0 {
			s.WriteString(": ")
		}
		s.WriteString(err.Error())
	}
	return s.String()
}
