package processing

import (
	"fmt"
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

var errFn = func(ss string) func(s string, p ...interface{}) errors.Err {
	fn := func(s string, p ...interface{}) errors.Err {
		return wrapErr(nil, fmt.Sprintf("%s: %s", ss, s), p...)
	}
	return fn
}

var ErrDuplicateObject = func(s string, p ...interface{}) errDuplicateKey {
	return errDuplicateKey{wrapErr(nil, fmt.Sprintf("Duplicate key: %s", s), p...)}
}
