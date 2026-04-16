package tests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

type livenessReproAttemptResult string

const (
	livenessReproWindowMiss livenessReproAttemptResult = "window_miss"
	livenessReproPass       livenessReproAttemptResult = "pass_no_repro"
	livenessReproFailure    livenessReproAttemptResult = "reproduced_liveness_failure"
)

type livenessReproAttemptSummary struct {
	Attempt         int
	Result          livenessReproAttemptResult
	CurrentHeight   uint64
	TargetHeight    uint64
	Checkpoint      uint64
	TargetSigner    common.Address
	TargetValidator common.Address
	Survivors       []common.Address
	BeforeProgress  uint64
	AfterProgress   uint64
	Evidence        testkit.ConsensusConflictEvidence
	Reason          string
	Err             error
}

type livenessReproRunSummary struct {
	MaxAttempts int
	Attempts    []livenessReproAttemptSummary
	WindowMiss  int
	PassNoRepro int
	Reproduced  int
}

type livenessStopWindow struct {
	CurrentHeight    uint64
	TargetHeight     uint64
	TargetSigner     common.Address
	TargetValidator  common.Address
	Survivors        []common.Address
	SurvivorHeads    []testkit.HeadSample
	TriggerHeads     []testkit.HeadSample
	TriggerConflicts []testkit.HeadHashConflict
	TriggerWindow    []testkit.TimedHeadConflictSample
}

type postStopClassification string

const (
	postStopConvergedImmediately postStopClassification = "converged_immediately"
	postStopAdvancedImmediately  postStopClassification = "advanced_immediately"
	postStopStillSplitSameHeight postStopClassification = "still_split_same_height"
	postStopSettledWithoutSplit  postStopClassification = "settled_without_split"
)

type postStopObservation struct {
	Classification postStopClassification
	SurvivorHeads  []testkit.HeadSample
	SplitRounds    int
	ProbeCount     int
	Reason         string
}

func summarizeWindowHasConflict(samples []testkit.TimedHeadConflictSample) bool {
	for _, sample := range samples {
		if len(sample.Conflicts) > 0 {
			return true
		}
	}
	return false
}

func otherValidators(validators []common.Address, exclude common.Address) []common.Address {
	items := make([]common.Address, 0, len(validators))
	for _, validator := range validators {
		if validator == exclude {
			continue
		}
		items = append(items, validator)
	}
	return items
}

func livenessReproMaxAttempts() int {
	if raw := os.Getenv("LIVENESS_REPRO_MAX_ATTEMPTS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return value
		}
	}
	return 5
}

func livenessReproWindowTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_WINDOW_TIMEOUT")); raw != "" {
		if value, err := time.ParseDuration(raw); err == nil && value > 0 {
			return value
		}
	}
	return testkit.LongWindowTimeout(6)
}

func livenessReproConfirmationProbes() int {
	if raw := os.Getenv("LIVENESS_REPRO_CONFIRMATION_PROBES"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 3 {
			return value
		}
	}
	return 3
}

func livenessReproConfirmationPoll() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_CONFIRMATION_POLL")); raw != "" {
		if value, err := time.ParseDuration(raw); err == nil && value > 0 {
			return value
		}
	}
	return 120 * time.Millisecond
}

func livenessReproPostStopProbes() int {
	if raw := os.Getenv("LIVENESS_REPRO_POST_STOP_PROBES"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 3 {
			return value
		}
	}
	return 5
}

func livenessReproPostStopPoll() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_POST_STOP_POLL")); raw != "" {
		if value, err := time.ParseDuration(raw); err == nil && value > 0 {
			return value
		}
	}
	return 120 * time.Millisecond
}

func livenessReproMinSplitRounds() int {
	if raw := os.Getenv("LIVENESS_REPRO_MIN_SPLIT_ROUNDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 2 {
			return value
		}
	}
	return 3
}

func livenessReproMinimumAttemptBudget() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("LIVENESS_REPRO_MIN_ATTEMPT_BUDGET")); raw != "" {
		if value, err := time.ParseDuration(raw); err == nil && value > 0 {
			return value
		}
	}
	return 45 * time.Second
}

func livenessReproAttemptBudget() time.Duration {
	window := livenessReproWindowTimeout()
	stallBudget := 45 * time.Second
	postStopBudget := time.Duration(livenessReproPostStopProbes()+2) * livenessReproPostStopPoll()
	confirmBudget := time.Duration(livenessReproConfirmationProbes()+2) * livenessReproConfirmationPoll()
	budget := window + stallBudget + postStopBudget + confirmBudget
	if budget < livenessReproMinimumAttemptBudget() {
		return livenessReproMinimumAttemptBudget()
	}
	return budget
}

func remainingTestBudget(t *testing.T) (time.Duration, bool) {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(deadline), true
}

func ensureAttemptBudget(t *testing.T, attempt int) error {
	t.Helper()
	remaining, ok := remainingTestBudget(t)
	if !ok {
		return nil
	}
	required := livenessReproAttemptBudget()
	if remaining <= required {
		return fmt.Errorf("remaining test budget %s is below required attempt budget %s", remaining.Truncate(time.Second), required.Truncate(time.Second))
	}
	return nil
}

func (s *livenessReproRunSummary) add(attempt livenessReproAttemptSummary) {
	s.Attempts = append(s.Attempts, attempt)
	switch attempt.Result {
	case livenessReproWindowMiss:
		s.WindowMiss++
	case livenessReproPass:
		s.PassNoRepro++
	case livenessReproFailure:
		s.Reproduced++
	}
}

func logLivenessReproRunSummary(t *testing.T, run livenessReproRunSummary) {
	t.Helper()
	t.Logf(
		"liveness repro summary: attempts=%d/%d window_miss=%d pass_no_repro=%d reproduced_liveness_failure=%d",
		len(run.Attempts),
		run.MaxAttempts,
		run.WindowMiss,
		run.PassNoRepro,
		run.Reproduced,
	)
	for _, attempt := range run.Attempts {
		t.Logf(
			"liveness repro attempt result: attempt=%d result=%s current_height=%d target_height=%d checkpoint=%d target_signer=%s target_validator=%s before=%d after=%d same_height_multi_hash=%t survivor_distinct_heads=%t target_is_next_in_turn=%t post_stop_same_height_split=%t post_stop_multi_hash=%t post_stop_classification=%s signed_recently=%t competing_keyword=%t reason=%s err=%v",
			attempt.Attempt,
			attempt.Result,
			attempt.CurrentHeight,
			attempt.TargetHeight,
			attempt.Checkpoint,
			attempt.TargetSigner.Hex(),
			attempt.TargetValidator.Hex(),
			attempt.BeforeProgress,
			attempt.AfterProgress,
			attempt.Evidence.SameHeightMultiHash,
			attempt.Evidence.SurvivorDistinctHeads,
			attempt.Evidence.TargetIsNextInTurn,
			attempt.Evidence.PostStopSameHeightSplit,
			attempt.Evidence.PostStopMultiHash,
			attempt.Evidence.PostStopClassification,
			attempt.Evidence.SignedRecentlyObserved,
			attempt.Evidence.CompetingBlockObserved,
			attempt.Reason,
			attempt.Err,
		)
	}
}

func findValidatorHeadSample(validator common.Address, samples []testkit.HeadSample) (testkit.HeadSample, bool) {
	rpcURL := strings.TrimSpace(ctx.ValidatorRPCByValidator(validator))
	if rpcURL == "" {
		return testkit.HeadSample{}, false
	}
	for _, sample := range samples {
		if strings.EqualFold(strings.TrimSpace(sample.URL), rpcURL) {
			return sample, true
		}
	}
	return testkit.HeadSample{}, false
}

func collectSurvivorHeadSamples(survivors []common.Address, samples []testkit.HeadSample) []testkit.HeadSample {
	items := make([]testkit.HeadSample, 0, len(survivors))
	for _, survivor := range survivors {
		sample, ok := findValidatorHeadSample(survivor, samples)
		if !ok {
			continue
		}
		items = append(items, sample)
	}
	return items
}

func confirmCompetingStopWindow(
	t *testing.T,
	targetHeight uint64,
	targetValidator common.Address,
	survivors []common.Address,
) (*livenessStopWindow, bool) {
	t.Helper()

	triggerHeads := testkit.ObserveHeads(ctx)
	triggerConflicts := testkit.SameHeightHashConflicts(triggerHeads)
	if len(triggerConflicts) == 0 {
		return nil, false
	}

	left, ok := findValidatorHeadSample(survivors[0], triggerHeads)
	if !ok || left.Error != "" || left.Height+1 != targetHeight {
		return nil, false
	}
	right, ok := findValidatorHeadSample(survivors[1], triggerHeads)
	if !ok || right.Error != "" || right.Height+1 != targetHeight {
		return nil, false
	}
	if left.Height == 0 || left.Height != right.Height || left.Hash == right.Hash {
		return nil, false
	}

	targetSample, ok := findValidatorHeadSample(targetValidator, triggerHeads)
	if ok && targetSample.Error == "" && targetSample.Height > targetHeight {
		return nil, false
	}

	return &livenessStopWindow{
		CurrentHeight:    left.Height,
		TargetHeight:     targetHeight,
		TargetValidator:  targetValidator,
		Survivors:        append([]common.Address(nil), survivors...),
		SurvivorHeads:    []testkit.HeadSample{left, right},
		TriggerHeads:     triggerHeads,
		TriggerConflicts: triggerConflicts,
	}, true
}

func stableSiblingWindowForTargetHeight(
	targetHeight uint64,
	targetValidator common.Address,
	survivors []common.Address,
	probes []testkit.TimedHeadConflictSample,
) (*livenessStopWindow, string) {
	requiredProbes := livenessReproConfirmationProbes()
	if len(probes) < requiredProbes {
		return nil, "need enough realtime probes to confirm dangerous window"
	}

	var chosen *livenessStopWindow
	var expectedCurrent uint64
	expectedConflict := targetHeight - 1

	for idx, probe := range probes {
		if len(probe.Conflicts) == 0 {
			return nil, "probe missing same-height multi-hash"
		}

		left, ok := findValidatorHeadSample(survivors[0], probe.Samples)
		if !ok || left.Error != "" {
			return nil, "left survivor head sample unavailable"
		}
		right, ok := findValidatorHeadSample(survivors[1], probe.Samples)
		if !ok || right.Error != "" {
			return nil, "right survivor head sample unavailable"
		}
		if left.Height == 0 || right.Height == 0 {
			return nil, "survivor head height is zero"
		}
		if left.Height != right.Height {
			return nil, "survivor heights diverged before stop"
		}
		if left.Height != expectedConflict {
			return nil, "survivor conflict height drifted away from target-1"
		}
		if left.Hash == right.Hash {
			return nil, "survivors already converged to same hash"
		}

		targetSample, ok := findValidatorHeadSample(targetValidator, probe.Samples)
		if ok && targetSample.Error == "" && targetSample.Height > expectedConflict {
			return nil, "target validator already advanced beyond competing window"
		}

		confirmed := false
		for _, conflict := range probe.Conflicts {
			if conflict.Height != expectedConflict {
				continue
			}
			if len(conflict.Hashes) < 2 {
				continue
			}
			hasLeft := false
			hasRight := false
			for _, sample := range probe.Samples {
				if sample.Error != "" || sample.Height != expectedConflict {
					continue
				}
				switch {
				case strings.EqualFold(strings.TrimSpace(sample.URL), strings.TrimSpace(left.URL)) && sample.Hash == left.Hash:
					hasLeft = true
				case strings.EqualFold(strings.TrimSpace(sample.URL), strings.TrimSpace(right.URL)) && sample.Hash == right.Hash:
					hasRight = true
				}
			}
			if hasLeft && hasRight {
				confirmed = true
				break
			}
		}
		if !confirmed {
			return nil, "same-height conflict did not preserve both survivor sibling hashes"
		}

		if idx == 0 {
			expectedCurrent = left.Height
			chosen = &livenessStopWindow{
				CurrentHeight:    left.Height,
				TargetHeight:     targetHeight,
				TargetValidator:  targetValidator,
				Survivors:        append([]common.Address(nil), survivors...),
				SurvivorHeads:    []testkit.HeadSample{left, right},
				TriggerHeads:     append([]testkit.HeadSample(nil), probe.Samples...),
				TriggerConflicts: append([]testkit.HeadHashConflict(nil), probe.Conflicts...),
			}
			continue
		}
		if left.Height != expectedCurrent {
			return nil, "conflicting sibling window advanced during confirmation probes"
		}
	}

	if chosen == nil {
		return nil, "missing initial confirmed stop window"
	}
	chosen.TriggerWindow = append([]testkit.TimedHeadConflictSample(nil), probes...)
	return chosen, ""
}

func classifyPostStopObservation(
	window *livenessStopWindow,
	probes []testkit.TimedHeadConflictSample,
) postStopObservation {
	if window == nil {
		return postStopObservation{
			Classification: postStopSettledWithoutSplit,
			Reason:         "missing stop window",
		}
	}
	if len(probes) == 0 {
		return postStopObservation{
			Classification: postStopSettledWithoutSplit,
			Reason:         "missing post-stop probes",
		}
	}

	finalHeads := make([]testkit.HeadSample, 0, len(window.Survivors))
	stillSplitCount := 0
	convergedCount := 0
	minSplitRounds := livenessReproMinSplitRounds()
	for _, probe := range probes {
		left, ok := findValidatorHeadSample(window.Survivors[0], probe.Samples)
		if !ok || left.Error != "" {
			return postStopObservation{
				Classification: postStopSettledWithoutSplit,
				Reason:         "left survivor sample unavailable after stop",
				SurvivorHeads:  collectSurvivorHeadSamples(window.Survivors, probe.Samples),
				ProbeCount:     len(probes),
			}
		}
		right, ok := findValidatorHeadSample(window.Survivors[1], probe.Samples)
		if !ok || right.Error != "" {
			return postStopObservation{
				Classification: postStopSettledWithoutSplit,
				Reason:         "right survivor sample unavailable after stop",
				SurvivorHeads:  collectSurvivorHeadSamples(window.Survivors, probe.Samples),
				ProbeCount:     len(probes),
			}
		}
		finalHeads = []testkit.HeadSample{left, right}

		if left.Height >= window.TargetHeight || right.Height >= window.TargetHeight {
			return postStopObservation{
				Classification: postStopAdvancedImmediately,
				Reason:         "chain kept growing immediately after stop",
				SurvivorHeads:  finalHeads,
				SplitRounds:    stillSplitCount,
				ProbeCount:     len(probes),
			}
		}

		if left.Height == window.CurrentHeight && right.Height == window.CurrentHeight && left.Hash != right.Hash {
			stillSplitCount++
			continue
		}
		if left.Height == right.Height && left.Hash == right.Hash {
			convergedCount++
			continue
		}
	}

	if stillSplitCount >= minSplitRounds {
		return postStopObservation{
			Classification: postStopStillSplitSameHeight,
			Reason:         "survivors remained on distinct sibling heads after stop across multiple probes",
			SurvivorHeads:  finalHeads,
			SplitRounds:    stillSplitCount,
			ProbeCount:     len(probes),
		}
	}
	if convergedCount > 0 {
		return postStopObservation{
			Classification: postStopConvergedImmediately,
			Reason:         "survivors converged to same hash before dangerous split could persist",
			SurvivorHeads:  finalHeads,
			SplitRounds:    stillSplitCount,
			ProbeCount:     len(probes),
		}
	}

	return postStopObservation{
		Classification: postStopSettledWithoutSplit,
		Reason:         "post-stop probes did not preserve dangerous same-height split long enough",
		SurvivorHeads:  finalHeads,
		SplitRounds:    stillSplitCount,
		ProbeCount:     len(probes),
	}
}

func waitForCompetingStopWindow(
	t *testing.T,
	signers []common.Address,
	activeValidators []common.Address,
	timeout time.Duration,
) (*livenessStopWindow, string) {
	t.Helper()

	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	deadline := time.Now().Add(timeout)
	poll := ctx.BlockPollInterval()
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}
	if poll > 300*time.Millisecond {
		poll = 300 * time.Millisecond
	}

	lastReason := "no same-height multi-hash observed"
	for time.Now().Before(deadline) {
		headSamples := testkit.ObserveHeads(ctx)
		conflicts := testkit.SameHeightHashConflicts(headSamples)
		if len(conflicts) == 0 {
			time.Sleep(poll)
			continue
		}

		var bestHeight uint64
		for _, conflict := range conflicts {
			if conflict.Height > bestHeight {
				bestHeight = conflict.Height
			}
		}
		targetHeight := bestHeight + 1
		targetSigner := testkit.SelectInTurnSignerAtHeight(signers, targetHeight)
		if targetSigner == (common.Address{}) {
			lastReason = "failed to select in-turn signer from conflicting window"
			time.Sleep(poll)
			continue
		}
		targetValidator, err := ctx.ValidatorAddressBySigner(targetSigner)
		if err != nil || targetValidator == (common.Address{}) {
			lastReason = "failed to resolve validator for in-turn signer"
			time.Sleep(poll)
			continue
		}
		survivors := otherValidators(activeValidators, targetValidator)
		if len(survivors) != 2 {
			lastReason = "expected exactly two survivors for candidate window"
			time.Sleep(poll)
			continue
		}

		first, ok := confirmCompetingStopWindow(t, targetHeight, targetValidator, survivors)
		if !ok {
			lastReason = "same-height multi-hash observed but survivors were not both on distinct sibling heads"
			time.Sleep(poll)
			continue
		}
		probeWindow := testkit.ObserveHeadConflictSamples(ctx, livenessReproConfirmationProbes(), livenessReproConfirmationPoll())
		second, reason := stableSiblingWindowForTargetHeight(targetHeight, targetValidator, survivors, probeWindow)
		if second == nil {
			lastReason = reason
			time.Sleep(poll)
			continue
		}

		second.TargetSigner = targetSigner
		if second.CurrentHeight < first.CurrentHeight {
			second.CurrentHeight = first.CurrentHeight
		}
		if len(second.TriggerHeads) == 0 {
			second.TriggerHeads = append([]testkit.HeadSample(nil), first.TriggerHeads...)
		}
		if len(second.TriggerConflicts) == 0 {
			second.TriggerConflicts = append([]testkit.HeadHashConflict(nil), first.TriggerConflicts...)
		}
		return second, ""
	}

	return nil, lastReason
}

func runLivenessReproAttempt(t *testing.T, attempt int) livenessReproAttemptSummary {
	t.Helper()

	summary := livenessReproAttemptSummary{Attempt: attempt}
	if err := ensureAttemptBudget(t, attempt); err != nil {
		summary.Result = livenessReproWindowMiss
		summary.Reason = err.Error()
		return summary
	}

	ensureMinActiveValidators(t, 3, 1)
	activeValidators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("get active validators failed: %v", err)
	}
	if len(activeValidators) != 3 {
		t.Fatalf("requires exactly 3 active validators, got %d", len(activeValidators))
	}

	signers := sortedRuntimeSigners(t)
	if len(signers) != 3 {
		t.Fatalf("requires exactly 3 runtime signers, got %d: %v", len(signers), signers)
	}

	epoch := uint64(0)
	if epochBig, err := ctx.Proposal.Epoch(nil); err == nil && epochBig != nil && epochBig.Sign() > 0 {
		epoch = epochBig.Uint64()
	}
	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read latest header failed: %v", err)
	}
	summary.CurrentHeight = head.Number.Uint64()
	if epoch > 0 {
		summary.Checkpoint = ((summary.CurrentHeight / epoch) + 1) * epoch
	}

	t.Logf(
		"liveness repro start: attempt=%d current_height=%d checkpoint=%d mode=realtime-observation remaining_budget=%v recent_coinbases=%v",
		attempt,
		summary.CurrentHeight,
		summary.Checkpoint,
		func() time.Duration {
			if remaining, ok := remainingTestBudget(t); ok {
				return remaining.Truncate(time.Second)
			}
			return 0
		}(),
		testkit.RecentCoinbases(ctx, 12),
	)
	stopWindow, missReason := waitForCompetingStopWindow(t, signers, activeValidators, livenessReproWindowTimeout())
	if stopWindow == nil {
		summary.Result = livenessReproWindowMiss
		summary.Reason = missReason
		t.Logf("liveness repro window miss: attempt=%d reason=%s", attempt, missReason)
		return summary
	}

	summary.TargetHeight = stopWindow.TargetHeight
	summary.TargetSigner = stopWindow.TargetSigner
	summary.TargetValidator = stopWindow.TargetValidator
	summary.Survivors = append([]common.Address(nil), stopWindow.Survivors...)

	targetSignerAddr, targetSignerKey := resolveValidatorSignerIdentity(t, stopWindow.TargetValidator)
	if targetSignerAddr != stopWindow.TargetSigner {
		t.Fatalf("runtime signer drifted: validator=%s runtime=%s expected=%s", stopWindow.TargetValidator.Hex(), targetSignerAddr.Hex(), stopWindow.TargetSigner.Hex())
	}

	switchObserverIfStoppingPrimaryValidator(t, stopWindow.TargetValidator)

	stopped := false
	defer func() {
		if !stopped {
			return
		}
		if err := testkit.RestartValidatorNodeWithSigner(ctx, stopWindow.TargetValidator, targetSignerKey, 90*time.Second); err != nil {
			t.Errorf("attempt=%d restart validator %s after liveness repro failed: %v", attempt, stopWindow.TargetValidator.Hex(), err)
		}
	}()

	t.Logf(
		"dangerous competing window hit: attempt=%d height=%d target_height=%d target_signer=%s target_validator=%s survivors=%v survivor_heads=%v trigger_conflicts=%v trigger_heads=%v",
		attempt,
		stopWindow.CurrentHeight,
		stopWindow.TargetHeight,
		stopWindow.TargetSigner.Hex(),
		stopWindow.TargetValidator.Hex(),
		stopWindow.Survivors,
		stopWindow.SurvivorHeads,
		stopWindow.TriggerConflicts,
		stopWindow.TriggerHeads,
	)
	t.Logf(
		"liveness repro stop target validator at competing pre-turn window: attempt=%d current_height=%d target_height=%d target_signer=%s target_validator=%s",
		attempt,
		stopWindow.CurrentHeight,
		stopWindow.TargetHeight,
		stopWindow.TargetSigner.Hex(),
		stopWindow.TargetValidator.Hex(),
	)

	if err := testkit.StopValidatorNode(ctx, stopWindow.TargetValidator, 30*time.Second); err != nil {
		t.Fatalf("stop target validator %s failed: %v", stopWindow.TargetValidator.Hex(), err)
	}
	stopped = true

	postStopWindow := testkit.ObserveHeadConflictSamples(ctx, livenessReproPostStopProbes(), livenessReproPostStopPoll())
	postStop := classifyPostStopObservation(stopWindow, postStopWindow)
	t.Logf(
		"post-stop observation: attempt=%d classification=%s split_rounds=%d/%d reason=%s survivor_heads=%v samples=%v",
		attempt,
		postStop.Classification,
		postStop.SplitRounds,
		postStop.ProbeCount,
		postStop.Reason,
		postStop.SurvivorHeads,
		postStopWindow,
	)

	beforeProgress, beforeSamples := testkit.ObserveHeights(ctx)
	summary.BeforeProgress = beforeProgress
	if beforeProgress >= stopWindow.TargetHeight+1 {
		summary.Result = livenessReproWindowMiss
		summary.Reason = "post-stop sampling shows target window already passed"
		summary.Evidence = testkit.CollectConsensusConflictEvidence(ctx, stopWindow.Survivors, stopWindow.TargetHeight, beforeSamples, beforeSamples, 8)
		summary.Evidence.TriggerHeadSamples = append([]testkit.HeadSample(nil), stopWindow.TriggerHeads...)
		summary.Evidence.TriggerConflicts = append([]testkit.HeadHashConflict(nil), stopWindow.TriggerConflicts...)
		summary.Evidence.TriggerWindowSamples = append([]testkit.TimedHeadConflictSample(nil), stopWindow.TriggerWindow...)
		summary.Evidence.PostStopWindowSamples = append([]testkit.TimedHeadConflictSample(nil), postStopWindow...)
		summary.Evidence.TriggerSurvivorHeads = append([]testkit.HeadSample(nil), stopWindow.SurvivorHeads...)
		summary.Evidence.PostStopSurvivorHeads = append([]testkit.HeadSample(nil), postStop.SurvivorHeads...)
		summary.Evidence.SameHeightMultiHash = len(stopWindow.TriggerConflicts) > 0
		summary.Evidence.PostStopMultiHash = summarizeWindowHasConflict(postStopWindow)
		summary.Evidence.SurvivorDistinctHeads = len(stopWindow.SurvivorHeads) == 2 && stopWindow.SurvivorHeads[0].Hash != stopWindow.SurvivorHeads[1].Hash
		summary.Evidence.TargetIsNextInTurn = stopWindow.TargetSigner != (common.Address{})
		summary.Evidence.PostStopSameHeightSplit = postStop.Classification == postStopStillSplitSameHeight
		summary.Evidence.PostStopClassification = string(postStop.Classification)
		t.Logf(
			"liveness repro window miss: attempt=%d target_height=%d before_progress=%d reason=%s evidence=%+v",
			attempt,
			stopWindow.TargetHeight,
			beforeProgress,
			summary.Reason,
			summary.Evidence,
		)
		return summary
	}
	switch postStop.Classification {
	case postStopConvergedImmediately, postStopSettledWithoutSplit:
		summary.Result = livenessReproWindowMiss
		summary.Reason = postStop.Reason
		summary.Evidence = testkit.CollectConsensusConflictEvidence(ctx, stopWindow.Survivors, stopWindow.TargetHeight, beforeSamples, beforeSamples, 8)
		summary.Evidence.TriggerHeadSamples = append([]testkit.HeadSample(nil), stopWindow.TriggerHeads...)
		summary.Evidence.TriggerConflicts = append([]testkit.HeadHashConflict(nil), stopWindow.TriggerConflicts...)
		summary.Evidence.TriggerWindowSamples = append([]testkit.TimedHeadConflictSample(nil), stopWindow.TriggerWindow...)
		summary.Evidence.PostStopWindowSamples = append([]testkit.TimedHeadConflictSample(nil), postStopWindow...)
		summary.Evidence.TriggerSurvivorHeads = append([]testkit.HeadSample(nil), stopWindow.SurvivorHeads...)
		summary.Evidence.PostStopSurvivorHeads = append([]testkit.HeadSample(nil), postStop.SurvivorHeads...)
		summary.Evidence.SameHeightMultiHash = len(stopWindow.TriggerConflicts) > 0
		summary.Evidence.PostStopMultiHash = summarizeWindowHasConflict(postStopWindow)
		summary.Evidence.SurvivorDistinctHeads = len(stopWindow.SurvivorHeads) == 2 && stopWindow.SurvivorHeads[0].Hash != stopWindow.SurvivorHeads[1].Hash
		summary.Evidence.TargetIsNextInTurn = stopWindow.TargetSigner != (common.Address{})
		summary.Evidence.PostStopSameHeightSplit = false
		summary.Evidence.PostStopClassification = string(postStop.Classification)
		t.Logf(
			"liveness repro window miss after stop: attempt=%d target_height=%d before_progress=%d reason=%s evidence=%+v",
			attempt,
			stopWindow.TargetHeight,
			beforeProgress,
			summary.Reason,
			summary.Evidence,
		)
		return summary
	case postStopAdvancedImmediately:
		summary.AfterProgress = beforeProgress
		summary.Result = livenessReproPass
		summary.Reason = postStop.Reason
		summary.Evidence = testkit.CollectConsensusConflictEvidence(ctx, stopWindow.Survivors, stopWindow.TargetHeight, beforeSamples, beforeSamples, 8)
		summary.Evidence.TriggerHeadSamples = append([]testkit.HeadSample(nil), stopWindow.TriggerHeads...)
		summary.Evidence.TriggerConflicts = append([]testkit.HeadHashConflict(nil), stopWindow.TriggerConflicts...)
		summary.Evidence.TriggerWindowSamples = append([]testkit.TimedHeadConflictSample(nil), stopWindow.TriggerWindow...)
		summary.Evidence.PostStopWindowSamples = append([]testkit.TimedHeadConflictSample(nil), postStopWindow...)
		summary.Evidence.TriggerSurvivorHeads = append([]testkit.HeadSample(nil), stopWindow.SurvivorHeads...)
		summary.Evidence.PostStopSurvivorHeads = append([]testkit.HeadSample(nil), postStop.SurvivorHeads...)
		summary.Evidence.SameHeightMultiHash = len(stopWindow.TriggerConflicts) > 0
		summary.Evidence.PostStopMultiHash = summarizeWindowHasConflict(postStopWindow)
		summary.Evidence.SurvivorDistinctHeads = len(stopWindow.SurvivorHeads) == 2 && stopWindow.SurvivorHeads[0].Hash != stopWindow.SurvivorHeads[1].Hash
		summary.Evidence.TargetIsNextInTurn = stopWindow.TargetSigner != (common.Address{})
		summary.Evidence.PostStopSameHeightSplit = false
		summary.Evidence.PostStopClassification = string(postStop.Classification)
		return summary
	}

	_, waitErr := testkit.WaitUntilHeightOrStall(
		ctx,
		"dangerous competing split after stopping in-turn signer",
		stopWindow.TargetHeight,
		15*time.Second,
		30*time.Second,
	)
	afterProgress, afterSamples := testkit.ObserveHeights(ctx)
	summary.AfterProgress = afterProgress
	summary.Evidence = testkit.CollectConsensusConflictEvidence(ctx, stopWindow.Survivors, stopWindow.TargetHeight, beforeSamples, afterSamples, 8)
	summary.Evidence.TriggerHeadSamples = append([]testkit.HeadSample(nil), stopWindow.TriggerHeads...)
	summary.Evidence.TriggerConflicts = append([]testkit.HeadHashConflict(nil), stopWindow.TriggerConflicts...)
	summary.Evidence.TriggerWindowSamples = append([]testkit.TimedHeadConflictSample(nil), stopWindow.TriggerWindow...)
	summary.Evidence.PostStopWindowSamples = append([]testkit.TimedHeadConflictSample(nil), postStopWindow...)
	summary.Evidence.TriggerSurvivorHeads = append([]testkit.HeadSample(nil), stopWindow.SurvivorHeads...)
	summary.Evidence.PostStopSurvivorHeads = append([]testkit.HeadSample(nil), postStop.SurvivorHeads...)
	summary.Evidence.SameHeightMultiHash = len(stopWindow.TriggerConflicts) > 0
	summary.Evidence.PostStopMultiHash = summarizeWindowHasConflict(postStopWindow)
	summary.Evidence.SurvivorDistinctHeads = len(stopWindow.SurvivorHeads) == 2 && stopWindow.SurvivorHeads[0].Hash != stopWindow.SurvivorHeads[1].Hash
	summary.Evidence.TargetIsNextInTurn = stopWindow.TargetSigner != (common.Address{})
	summary.Evidence.PostStopSameHeightSplit = postStop.Classification == postStopStillSplitSameHeight
	summary.Evidence.PostStopClassification = string(postStop.Classification)

	if waitErr != nil {
		summary.Result = livenessReproFailure
		summary.Err = waitErr
		summary.Reason = "chain stalled after stopping in-turn signer while survivors remained on distinct sibling heads"
		return summary
	}

	summary.Result = livenessReproPass
	summary.Reason = "chain recovered from dangerous competing split and kept growing"
	return summary
}

func TestZ_Liveness_StopInTurnSigner_DuringCompetingWindow(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if !testkit.IsMultiValidatorSeparatedMode(ctx, 3) {
		t.Skip("requires multi-validator separated-signer topology")
	}

	netemCfg := testkit.LoopbackNetemConfigFromEnv()
	if netemCfg.Enabled {
		activeValidators, err := ctx.Validators.GetActiveValidators(nil)
		if err != nil {
			t.Fatalf("get active validators for loopback netem failed: %v", err)
		}
		clearNetem, ports, err := testkit.ApplyLoopbackNetemForValidators(ctx, activeValidators, netemCfg)
		if err != nil {
			t.Fatalf("apply loopback netem failed: %v", err)
		}
		defer func() {
			if err := clearNetem(); err != nil {
				t.Errorf("clear loopback netem failed: %v", err)
			}
		}()
		t.Logf("liveness repro loopback netem enabled: %s validator_p2p_ports=%v", netemCfg.Summary(), ports)
	}

	maxAttempts := livenessReproMaxAttempts()
	runSummary := livenessReproRunSummary{MaxAttempts: maxAttempts}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		summary := runLivenessReproAttempt(t, attempt)
		runSummary.add(summary)

		switch summary.Result {
		case livenessReproWindowMiss:
			continue
		case livenessReproFailure:
			logLivenessReproRunSummary(t, runSummary)
			t.Fatalf(
				"reproduced consensus liveness candidate: attempt=%d stopped_in_turn_signer=%s stopped_validator=%s competing_block_observed=%t survivor_waiting_recently_signed=%t height_stalled_at=%d target_height=%d reason=%s err=%v evidence=%+v",
				summary.Attempt,
				summary.TargetSigner.Hex(),
				summary.TargetValidator.Hex(),
				summary.Evidence.CompetingBlockObserved,
				summary.Evidence.SignedRecentlyObserved,
				summary.BeforeProgress,
				summary.TargetHeight,
				summary.Reason,
				summary.Err,
				summary.Evidence,
			)
		case livenessReproPass:
			continue
		default:
			t.Fatalf("unexpected liveness repro attempt result: %+v", summary)
		}
	}

	logLivenessReproRunSummary(t, runSummary)
	t.Logf(
		"liveness repro did not trigger stall across %d attempts: window_miss=%d pass_no_repro=%d",
		maxAttempts,
		runSummary.WindowMiss,
		runSummary.PassNoRepro,
	)
}
