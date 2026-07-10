package provider

import (
	"errors"
	"net/http"

	mailtrap "github.com/mailtrap/mailtrap-go"
)

// isNotFound reports whether err is a Mailtrap 404, so a resource deleted
// out-of-band is dropped from state on the next Read instead of erroring.
// TODO: switch to errors.As(err, &*mailtrap.NotFoundError{}) once that ships.
func isNotFound(err error) bool {
	var e *mailtrap.Error
	return errors.As(err, &e) && e.StatusCode == http.StatusNotFound
}
