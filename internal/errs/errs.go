package errs

import (
	"fmt"
)

func Wrap(sentinel, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", sentinel, err)
}

func WrapMsg(sentinel error, msg string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", sentinel, msg)
	}
	return fmt.Errorf("%w: %s: %v", sentinel, msg, err)
}
