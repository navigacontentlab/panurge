package pt

import (
	"errors"
	"testing"

	"github.com/twitchtv/twirp"
)

func ExpectTwirpInvalidArgument(t *testing.T, err error, argument string) {
	t.Helper()

	te, ok := checkTwirpErrorCode(t, err, twirp.InvalidArgument)
	if !ok {
		return
	}

	if te.Meta("argument") != argument {
		t.Errorf("expected validation to fail for the argument %q, got %q",
			argument, te.Meta("argument"))

		return
	}

	if testing.Verbose() {
		t.Logf("got expected invalid argument %q", argument)
	}
}

//nolint:ireturn
func checkTwirpErrorCode(t *testing.T, err error, code twirp.ErrorCode) (twirp.Error, bool) {
	t.Helper()

	if err == nil {
		t.Error("expected operation to fail")

		return nil, false
	}

	var twErr twirp.Error
	if !errors.As(err, &twErr) {
		t.Error("expected a twirp.Error")

		return nil, false
	}

	if twErr.Code() != code {
		t.Errorf("expected twirp error code %q, got %q", code, twErr.Code())

		return twErr, false
	}

	if testing.Verbose() {
		t.Logf("got expected Twirp error code %q", code)
	}

	return twErr, true
}

func CheckTwirpErrorCode(t *testing.T, err error, code twirp.ErrorCode) {
	t.Helper()

	_, _ = checkTwirpErrorCode(t, err, code)
}
