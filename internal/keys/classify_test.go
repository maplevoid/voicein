package keys

import (
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	tap := 300 * time.Millisecond
	short := 80 * time.Millisecond
	long := 400 * time.Millisecond

	cases := []struct {
		name                     string
		mode                     string
		down, recording, latched bool
		held                     time.Duration
		want                     Action
	}{
		{"hybrid idle down starts", "hybrid", true, false, false, 0, ActionStart},
		{"hybrid short up latches", "hybrid", false, true, false, short, ActionLatch},
		{"hybrid long up stops", "hybrid", false, true, false, long, ActionStop},
		{"hybrid latched up ignores", "hybrid", false, true, true, short, ActionNone},
		{"hybrid latched down stops", "hybrid", true, true, true, 0, ActionStop},
		{"hybrid hold-candidate down ignores", "hybrid", true, true, false, long, ActionNone},
		{"hybrid idle up ignores", "hybrid", false, false, false, 0, ActionNone},
		{"hold down starts", "hold", true, false, false, 0, ActionStart},
		{"hold up stops", "hold", false, true, false, short, ActionStop},
		{"hold second down ignores", "hold", true, true, false, 0, ActionNone},
		{"toggle down starts", "toggle", true, false, false, 0, ActionStart},
		{"toggle up ignores", "toggle", false, true, false, long, ActionNone},
		{"toggle second down stops", "toggle", true, true, false, 0, ActionStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.mode, tc.down, tc.recording, tc.latched, tc.held, tap)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
