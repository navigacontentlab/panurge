package navigaid

import (
	"net/http"
	"testing"
)

func TestGetAuthToken(t *testing.T) {
	samples := map[string]struct {
		Input string
		Token string
		Fail  bool
	}{
		"OmittedToken": {Input: "Bearer ", Fail: true},
		"OnlyBearer":   {Input: "Bearer", Fail: true},
		"LowerBearer":  {Input: "bearer xxx", Token: "xxx"},
		"UpperBearer":  {Input: "Bearer yyy", Token: "yyy"},
		"MixyBearer":   {Input: "BeaReR zzz", Token: "zzz"},
		"StrangeToken": {Input: "Bearer zzz./ foo", Token: "zzz./ foo"},
		"NotBearer":    {Input: "Random token", Fail: true},
	}

	for name := range samples {
		tc := samples[name]

		t.Run(name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("Authorization", tc.Input)

			token, err := getAuthToken(header)
			if err != nil {
				if tc.Fail {
					return
				}

				t.Fatalf("failed to parse input: %v", err)
			}

			if tc.Fail {
				t.Fatalf("did not fail as expected")
			}

			if token != tc.Token {
				t.Fatalf("wanted the token %q, got %q",
					tc.Token, token)
			}
		})
	}
}
