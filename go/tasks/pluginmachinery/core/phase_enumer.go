// Code generated by "enumer -type=Phase"; DO NOT EDIT.

//
package core

import (
	"fmt"
)

const _PhaseName = "PhaseUndefinedPhaseNotReadyPhaseWaitingForResourcesPhaseQueuedPhaseInitializingPhaseRunningPhaseSuccessPhaseRetryableFailurePhasePermanentFailure"

var _PhaseIndex = [...]uint8{0, 14, 27, 51, 62, 79, 91, 103, 124, 145}

func (i Phase) String() string {
	if i < 0 || i >= Phase(len(_PhaseIndex)-1) {
		return fmt.Sprintf("Phase(%d)", i)
	}
	return _PhaseName[_PhaseIndex[i]:_PhaseIndex[i+1]]
}

var _PhaseValues = []Phase{0, 1, 2, 3, 4, 5, 6, 7, 8}

var _PhaseNameToValueMap = map[string]Phase{
	_PhaseName[0:14]:    0,
	_PhaseName[14:27]:   1,
	_PhaseName[27:51]:   2,
	_PhaseName[51:62]:   3,
	_PhaseName[62:79]:   4,
	_PhaseName[79:91]:   5,
	_PhaseName[91:103]:  6,
	_PhaseName[103:124]: 7,
	_PhaseName[124:145]: 8,
}

// PhaseString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func PhaseString(s string) (Phase, error) {
	if val, ok := _PhaseNameToValueMap[s]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to Phase values", s)
}

// PhaseValues returns all values of the enum
func PhaseValues() []Phase {
	return _PhaseValues
}

// IsAPhase returns "true" if the value is listed in the enum definition. "false" otherwise
func (i Phase) IsAPhase() bool {
	for _, v := range _PhaseValues {
		if i == v {
			return true
		}
	}
	return false
}
