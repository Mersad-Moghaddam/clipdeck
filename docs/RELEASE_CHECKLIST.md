# v0.4 release gate — NOT READY TO PUBLISH

- [x] Baseline tests/vet/race recorded.
- [x] Filtered query API and mixed-case/filter argument regressions tested.
- [x] Portable CLI command wiring and safe row helper tests.
- [x] Stable recent-order index checked with EXPLAIN and benchmark experiment.
- [x] Installer dry-run network regression and missing argument checks.
- [ ] Complete planned query/CLI/privacy/concurrency test matrix.
- [ ] Common-match search performance gate (currently ~40–50 ms, not met).
- [ ] Full capture subprocess lifecycle hardening and attribution privacy policy.
- [ ] Unique-request paste failure correlation and lifecycle management.
- [ ] Old-daemon fallback and complete QML cache/input/layout verification.
- [ ] Installer staging/rollback/archive validation and complete shell tests.
- [ ] CI draft-release/package gating and distro install/upgrade verification.
- [ ] Native/emulated arm64 smoke testing.
- [ ] Fresh whole-branch code review and fixes.
- [ ] Version bump/tag/publish and independent public-download verification.
- [ ] Authorized local update and synthetic live smoke tests.
- [ ] Eligible directory/community submissions with actual URLs.

Do not tag, upload or announce this work as a completed faster release while
these gates remain open. Existing v0.3.1 is unchanged. No live configuration or
user history has been modified by these development commits.
