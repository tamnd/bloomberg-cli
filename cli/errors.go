package cli

import (
	"errors"

	"github.com/tamnd/bloomberg-cli/bloomberg"
)

func isNotFound(err error) bool {
	return errors.Is(err, bloomberg.ErrUnknownSection)
}
