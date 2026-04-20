package main

import "testing"

func TestBuildMaxTPSSteps(t *testing.T) {
	steps, err := buildMaxTPSSteps(1000, 100, 1300)
	if err != nil {
		t.Fatalf("buildMaxTPSSteps returned error: %v", err)
	}
	want := []int{1000, 1100, 1200, 1300}
	if len(steps) != len(want) {
		t.Fatalf("unexpected step count: got=%d want=%d (%v)", len(steps), len(want), steps)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("unexpected step at %d: got=%d want=%d", i, steps[i], want[i])
		}
	}
}

func TestBuildMaxTPSStepsIncludesTargetWhenStepMissesExactTarget(t *testing.T) {
	steps, err := buildMaxTPSSteps(1000, 300, 1700)
	if err != nil {
		t.Fatalf("buildMaxTPSSteps returned error: %v", err)
	}
	want := []int{1000, 1300, 1600, 1700}
	if len(steps) != len(want) {
		t.Fatalf("unexpected step count: got=%d want=%d (%v)", len(steps), len(want), steps)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("unexpected step at %d: got=%d want=%d", i, steps[i], want[i])
		}
	}
}

func TestRecommendedSenderAccountCount(t *testing.T) {
	cases := []struct {
		maxTPS int
		want   int
	}{
		{maxTPS: 1, want: 4},
		{maxTPS: 400, want: 4},
		{maxTPS: 450, want: 5},
		{maxTPS: 5000, want: 50},
		{maxTPS: 50000, want: 128},
	}
	for _, tc := range cases {
		if got := recommendedSenderAccountCount(tc.maxTPS); got != tc.want {
			t.Fatalf("recommendedSenderAccountCount(%d) = %d, want %d", tc.maxTPS, got, tc.want)
		}
	}
}
