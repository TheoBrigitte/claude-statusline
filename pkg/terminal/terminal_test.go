package terminal

import (
	"os"
	"testing"
)

// TestWidthFromEnv covers the COLUMNS fallback, which is what a status line
// ends up on whenever /dev/tty reports no size.
func TestWidthFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		set     bool
		columns string
		want    int
		wantErr bool
	}{
		{name: "valid width", set: true, columns: "132", want: 132},
		{name: "unset", set: false, wantErr: true},
		{name: "empty", set: true, columns: "", wantErr: true},
		{name: "not a number", set: true, columns: "wide", wantErr: true},
		{name: "zero", set: true, columns: "0", wantErr: true},
		{name: "negative", set: true, columns: "-20", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("COLUMNS", tt.columns)
			} else {
				// t.Setenv registers the restore, then unset for real so
				// LookupEnv reports the variable as absent.
				t.Setenv("COLUMNS", "")
				if err := os.Unsetenv("COLUMNS"); err != nil {
					t.Fatal(err)
				}
			}

			got, err := widthFromEnv()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got width %d, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// TestWidthIsUsableOrErrors checks the contract main relies on: Width either
// reports a positive width or an error, never a zero that would collapse the
// layout.
func TestWidthIsUsableOrErrors(t *testing.T) {
	w, err := Width()
	if err == nil && w <= 0 {
		t.Errorf("got width %d with no error, want a positive width", w)
	}
}

func BenchmarkWidth(b *testing.B) {
	for b.Loop() {
		Width() //nolint
	}
}
