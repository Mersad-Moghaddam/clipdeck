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

Development CLI cross-builds succeeded for amd64 and arm64 and `file` reports
both statically linked. Their version remains v0.3.1 until the planned release
bump; they are not published v0.4 artifacts. Docker tests are blocked by socket
permission denial; no privilege changes were attempted. CI now derives Go from
go.mod, tests installer/workflows, pins nfpm and stages tag releases as drafts;
complete package validation/action revision pinning remain outstanding.

Local development `make packages VERSION=0.4.0-dev` succeeded with nfpm 2.48.0
for amd64 deb/rpm/apk/tar. CLI reports 0.4.0-dev; both binaries are static,
tar entries are the two expected executables and its SHA256 verifies. Archive
inspection found expected binaries/unit/docs/license in packages; native distro
install/upgrade testing remains blocked. Installer now rejects unexpected or
nonregular tar entries, with an isolated malicious-symlink regression test.
