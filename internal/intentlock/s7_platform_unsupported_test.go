//go:build android || ios || (!linux && !darwin)

package intentlock

import "testing"

func TestS7UnsupportedPlatformRows(t *testing.T) {
	t.Run("PIB-409", func(t *testing.T) {
		if AuthoritySupported {
			t.Fatal("PIB-409 unsupported build reports mutation support")
		}
		authority, err := Acquire("PIB-409-path-must-not-be-opened")
		if authority != nil {
			t.Fatal("PIB-409 unsupported build returned an authority")
		}
		typed, ok := err.(*Error)
		if !ok || typed.Code != CodePrepareUnsupportedPlatform {
			t.Fatalf("PIB-409 unsupported error = %#v", err)
		}
	})
	t.Run("PIB-417", func(t *testing.T) {
		if AuthoritySupported {
			t.Fatal("PIB-417 unsupported build reports mutation support")
		}
		authority, err := Acquire("PIB-417-path-must-not-be-opened")
		if authority != nil {
			t.Fatal("PIB-417 unsupported build returned an authority")
		}
		typed, ok := err.(*Error)
		if !ok || typed.Code != CodePrepareUnsupportedPlatform {
			t.Fatalf("PIB-417 unsupported error = %#v", err)
		}
	})
}
