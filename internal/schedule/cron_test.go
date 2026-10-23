package schedule

import (
	"testing"
	"time"
)

func TestNextPresets(t *testing.T) {
	base := time.Date(2026, 10, 24, 10, 7, 0, 0, time.UTC)
	cases := []struct {
		expression string
		want       time.Time
	}{
		{"*/15 * * * *", time.Date(2026, 10, 24, 10, 15, 0, 0, time.UTC)},
		{"*/30 * * * *", time.Date(2026, 10, 24, 10, 30, 0, 0, time.UTC)},
		{"0 * * * *", time.Date(2026, 10, 24, 11, 0, 0, 0, time.UTC)},
		{"0 */12 * * *", time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)},
		{"0 0 * * *", time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := Next(tc.expression, base)
		if err != nil {
			t.Fatalf("%q: %v", tc.expression, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("%q: got %v want %v", tc.expression, got, tc.want)
		}
	}
}

func TestValidateNormalizesWhitespace(t *testing.T) {
	if err := Validate("  0   */12 * *  * "); err != nil {
		t.Fatal(err)
	}
	if got := Normalize("  0   */12 * *  * "); got != "0 */12 * * *" {
		t.Fatalf("normalize = %q", got)
	}
}

func TestValidateRejectsInvalidExpressions(t *testing.T) {
	for _, expression := range []string{"", "not-a-cron", "0 0 * * * *", "*/0 * * * *"} {
		if err := Validate(expression); err == nil {
			t.Fatalf("expected %q to be rejected", expression)
		}
	}
}
