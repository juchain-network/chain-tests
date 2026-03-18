# Release Checklist

## 1. Freeze
- [ ] Freeze target commit SHA and tag candidate.
- [ ] Confirm upstream dependencies (`chain`, `chain-contract`) commit SHAs.
- [ ] Confirm no untracked local patches in release branch.

## 2. Baseline validation
- [ ] `make precheck`
- [ ] `make ci PROFILE=release` (smoke + fork + posa)

## 3. Full regression and reports
- [ ] `make test-regression SCOPE=full`
- [ ] Verify `reports/regression_*/index.md` exists.
- [ ] Verify per-run `report.md`, `summary.json`, `manifest.json` exist.
- [ ] Confirm failed cases (if any) have reproducible command.

## 4. Performance/soak gate
- [ ] `make test-perf MODE=tiers`
- [ ] Review `verdict.json` thresholds:
  - [ ] success_rate >= 0.99
  - [ ] max_height_lag <= 8
  - [ ] stall_window <= 15s
  - [ ] p95 RPC latency <= 500ms
- [ ] Weekly soak latest run reviewed.

## 5. Risk and rollout
- [ ] Document known risks and non-blocking failures.
- [ ] Define rollout scope (canary percentage or environment ring).
- [ ] Define rollback trigger conditions.
- [ ] Confirm rollback steps tested from runbook.

## 6. Sign-off
- [ ] QA sign-off
- [ ] Consensus/Protocol sign-off
- [ ] DevOps sign-off
- [ ] Release manager final approval

## 7. Post-release
- [ ] Archive release reports and artifacts.
- [ ] Create release note with test evidence links.
- [ ] Open follow-up issues for deferred risks.
