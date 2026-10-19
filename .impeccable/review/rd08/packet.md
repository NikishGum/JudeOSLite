# RD-08 review packet

Project `/workspace/JudeOSLite`, branch `codex/rd08-pride-screenshots`, issue#200.
Read-only review; write only this folder's review.md. Do not edit code or assign issues.

## Original request and settled scope

Russian user provided three PRIDE CLUB OS screenshots and asked for the entire app
to follow them using Impeccable and shadcn/ui, add missing sections, update existing
ones, and create repository issues. Reference: cool light background, white panels,
crimson active Sidebar, header with context/search/profile, four real metrics,
overview + upcoming workouts, readable family/child/trainer cards with tabs,
payments/subscriptions, notifications, reports, settings, mobile bottom navigation.
Source reference images exist in chat only; pixel-level comparison by the fresh
reviewer is unavailable. Use the extracted composition and direction contract;
report review at that scope, not as pixel fidelity certification.

Eight user bugs: rectangular red pointer/programmatic focus; trainer branch selector;
green presence CTA turning gray until scroll; zero-unmarked journal close wording;
desktop-only collapse preference shown on phone; wrong calendar day/week/period
state animation; miscentered date arrows; Sidebar labels squashing during toggle.

User clarified: new module FRONTENDS are in this chat. Backend #201–#204 is for a
second developer working concurrently. Contracts already main cf0948f, #201 is
claimed by NikishGum. Do not implement backend here. Accounts are issued by admin.
iPay only prepared UI, actual connection later. No fake successful writes, online
payment or production demo data. All new UI screenshots use explicit synthetic
contract fixtures; legacy regressions use real Go/PostgreSQL. #188 physical/staff
acceptance remains separate.

## Artifact and constraints

Code: frontend/src/app/, frontend/src/features/club-workspace/, modified dashboard,
attendance, calendar and trainer components. Existing server, migrations, auth,
CSRF, guards, versions, lifecycle retained. Native dialogs keep full original forms.
Original local PRIDE logo and Rubik are retained, mock's different emblem is not a
provided brand asset. Backend contracts: docs/backend-contracts/README.md.
PRODUCT.md and DESIGN.md describe the previous runtime; documenter will merge
approved screenshot-system changes after review (redesign authorized explicitly).
Direction: .impeccable/surfaces/frontend-src-app-app-tsx.md. Code-led, user-pinned.

## Required evidence

All paths relative to this folder, see screenshots.json for provenance/hashes.
Desktop1440, mobile390 and narrow320 are required viewport classes. Open the
named files and verify capture validity before reviewing:
home-1440/390, families-1440/390, family-1440, member-1440,
member-Документы-390, payments-1440/390, payment-editor-390, plans-1440,
notifications-1440/390, settings-320, parent-family-390, parent-payments-390,
attendance-390, journal-complete-390, unavailable-320, finance-report-1440 (all.png).
Additional state captures `/tmp/rd08-contract-chromium/` if needed. Native dialog
captures show current viewport; other captures are viewport rather than whole-page.
No approved comp/build-spec/state machine, because precise user-pinned code-led UI.

## Checks and limits

Read chromium-contracts.json, webkit-contracts.json and logs. New frontend contracts
PASS both engines,22 state groups, moneyMinor/idempotency key retry, document upload
request/download URL (binary backend deferred), read-all, attendance next/complete;
keyboard/programmatic focus, calendar states/date centers, collapse intermediate
frames/main alignment/no-wrap and expanding endpoint. Real backend Chromium PWA,
training/reports/dashboard passed; WebKit reports/dashboard passed. Remaining PWA/
training harness runs are in progress; do not label them PASS yet.

Impeccable launcher was unavailable; context fallback read project files. One detector
attempt returned127 (logs/impeccable-detector.log); detector did NOT run. Apply the
craft-floor manually using cloud skill c6/impeccable reference/craft-floor.md.
React best-practices skill was read/applied. No new shipping raster: only existing
logo/fonts with their provenance. Do not claim real backend, real online payments,
physical installation, staff acceptance or frame performance beyond measured evidence.

## Required return

Act as impeccable_finish_reviewer. Load the skill finish-reviewer reference through
skills if available. Return five contract sections: disposition (ship/fix/rebuild/
recapture), fidelity/direction, material findings with exact files/lines or image
regions, craft/behavior evidence, and limits. Prioritize concrete defects; avoid
taste changes or expanding scope. If fixes are owed, one batch then verdict pass.
