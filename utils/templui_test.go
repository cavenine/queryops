package utils

import (
	"testing"

	"github.com/a-h/templ"
)

func TestMergeAttributes(t *testing.T) {
	t.Run("later values override earlier ones", func(t *testing.T) {
		got := MergeAttributes(
			templ.Attributes{"class": "btn", "data-role": "primary"},
			templ.Attributes{"class": "btn btn-primary", "disabled": true},
		)

		if got["class"] != "btn btn-primary" {
			t.Fatalf("class = %v, want %q", got["class"], "btn btn-primary")
		}
		if got["data-role"] != "primary" {
			t.Fatalf("data-role = %v, want %q", got["data-role"], "primary")
		}
		if got["disabled"] != true {
			t.Fatalf("disabled = %v, want %v", got["disabled"], true)
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		got := MergeAttributes()
		if len(got) != 0 {
			t.Fatalf("len(got) = %d, want 0", len(got))
		}
	})
}
