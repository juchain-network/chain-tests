package forkcap

import (
	"fmt"
	"strings"
)

type Phase string

const (
	PhasePre  Phase = "pre"
	PhasePost Phase = "post"
)

func NormalizePhase(raw string) Phase {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(PhasePre):
		return PhasePre
	case string(PhasePost):
		return PhasePost
	default:
		return ""
	}
}

func (p Phase) Valid() bool {
	return p == PhasePre || p == PhasePost
}

type GateExpectation struct {
	Phase      Phase
	ShouldFail bool
}

func ExpectationForPhase(phase Phase) (GateExpectation, error) {
	switch phase {
	case PhasePre:
		return GateExpectation{Phase: phase, ShouldFail: true}, nil
	case PhasePost:
		return GateExpectation{Phase: phase, ShouldFail: false}, nil
	default:
		return GateExpectation{}, fmt.Errorf("unsupported fork capability phase %q", phase)
	}
}

func ForkIncludesCapability(selectedFork, minimumFork string) (bool, error) {
	selectedFork = NormalizeFork(selectedFork)
	minimumFork = NormalizeFork(minimumFork)
	selectedIdx, selectedOK := ForkOrderIndex(selectedFork)
	if !selectedOK {
		return false, fmt.Errorf("unsupported selected fork %q", selectedFork)
	}
	minimumIdx, minimumOK := ForkOrderIndex(minimumFork)
	if !minimumOK {
		return false, fmt.Errorf("unsupported capability minimum fork %q", minimumFork)
	}
	return selectedIdx >= minimumIdx, nil
}

func ExpectationForCapability(selectedFork string, phase Phase, minimumFork string) (GateExpectation, error) {
	includes, err := ForkIncludesCapability(selectedFork, minimumFork)
	if err != nil {
		return GateExpectation{}, err
	}
	if !includes {
		return GateExpectation{}, fmt.Errorf("capability gated out: selected fork %q does not include minimum fork %q", selectedFork, minimumFork)
	}
	base, err := ExpectationForPhase(phase)
	if err != nil {
		return GateExpectation{}, err
	}
	if phase == PhasePost {
		return base, nil
	}
	base.ShouldFail = NormalizeFork(selectedFork) == NormalizeFork(minimumFork)
	return base, nil
}

func IsExecutionReject(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "invalid opcode") ||
		strings.Contains(msg, "execution reverted") ||
		strings.Contains(msg, "vm execution error") ||
		strings.Contains(msg, "invalid jump") ||
		strings.Contains(msg, "out of gas") ||
		strings.Contains(msg, "notactivated") ||
		strings.Contains(msg, "not activated")
}
