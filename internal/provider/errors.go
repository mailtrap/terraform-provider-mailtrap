package provider

import (
	"errors"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

// isNotFound reports whether err is a Mailtrap 404, so a resource deleted
// out-of-band is dropped from state on the next Read instead of erroring.
func isNotFound(err error) bool {
	var e *mailtrap.NotFoundError
	return errors.As(err, &e)
}
