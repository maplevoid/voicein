package keys

import "time"

// Action is what a hotkey edge should do to the current take.
type Action int

const (
	ActionNone Action = iota
	ActionStart
	ActionLatch
	ActionStop
)

// Classify maps a chord edge onto start / latch / stop.
//
// hybrid: press starts recording. A release shorter than tap latches
// toggle; a longer hold stops on release. A later press stops a latched take.
// hold: press starts, release stops.
// toggle: press starts, next press stops. Release is ignored.
func Classify(mode string, down, recording, latched bool, held, tap time.Duration) Action {
	switch mode {
	case "hold":
		if down && !recording {
			return ActionStart
		}
		if !down && recording {
			return ActionStop
		}
		return ActionNone
	case "hybrid":
		if down {
			if !recording {
				return ActionStart
			}
			if latched {
				return ActionStop
			}
			return ActionNone
		}
		if recording && !latched {
			if tap > 0 && held < tap {
				return ActionLatch
			}
			return ActionStop
		}
		return ActionNone
	default:
		if down {
			if recording {
				return ActionStop
			}
			return ActionStart
		}
		return ActionNone
	}
}
