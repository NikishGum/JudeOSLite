# Independent Impeccable documentation review · #214

Reviewed 20 October 2026 against the current working tree on `f34fed6`, including
the uncommitted F1–F3 corrections and refreshed validation document.

**Outcome: documentation check complete; preserve the incumbent design system.**
This is an ordinary Operate extension of PRIDE, with no new visual world or assets.
No remaining material mismatch was found in the two Groups documents at this snapshot.

## Evidence and conformance

Read the cloud Impeccable `reference/document.md` and new-work handoff guidance;
`PRODUCT.md`, `DESIGN.md`, `.impeccable/design.json`, `design-system/README.md`,
the Groups direction contract, `finish-review.md`, `verdict-pass.md`,
`targeted-pass.log`, all current `frontend/src/features/groups/` files, and
`docs/groups-workspace.md` / `docs/groups-workspace-validation.md`.
Also checked the existing NavigationBoundary/App integration, SessionForm and
runtime CSS sources for the documented dependencies and visual inheritance.

- **Visual system:** Groups inherits Rubik, runtime PRIDE red, white panels,
  semantic surfaces, controls and quiet dividers. Its local layout, responsive
  list/detail composition and focus scroll margin require no shared token or
  system-component change. Preserve `DESIGN.md`, its sidecar, global tokens and
  asset provenance. Historical RD-07/source values remain distinguished from
  current RD-08 runtime values; no pre-existing drift was repaired.
- **Context and guards:** group/club IDs drive requests and selection; names are
  display text. Tab, roster date and period start are stored by group ID in
  component memory. Existing guarded navigation protects forms; no personal
  drafts are placed in storage. Dated membership operations retain versions,
  explicit effective dates and history.
- **Partial additions:** the document accurately describes independent commands,
  named per-person results, success removal from the retry set, double-submit
  protection and pending lifecycle. Lost responses reconcile by group/from/until;
  unreadable history leaves an unknown result and blocks retry until checked.
  Unmount stops subsequent commands. No atomic batch or bulk transfer is claimed.
- **Schedule:** current rules remain visible alongside separately labelled future
  rules; the compact selector counts additional current intervals and the full
  header lists them. The local editor uses existing preview tokens and versions.
  Rules precede the selected group's 28-day sessions. Contextual one-date actions
  reuse SessionForm, retain actual session context and stored timezone, and refresh
  the period after a successful distant move or one-off creation.
- **Boundary:** `git diff 9ca620d` shows production changes confined to Groups.
  Sessions UI/routes/navigation, shared editors/CSS/tokens, API/backend and
  migrations have no diff. The existing link to group sessions remains. E2E
  selector updates are test maintenance; the documents make no new notification
  delivery, PWA/device or staff-acceptance claim.

## Validation wording and limits

The initial validation read contained stale PENDING/RUNNING entries. The parent
updated them during this review; a second read confirms targeted PASS evidence,
the F1–F3 **SHIP** verdict at its three-fix scope, and the old `da51751` CI failure
on obsolete Groups selectors. Latest-code mandatory CI remains pending and is
neither passed nor waived here.

This reviewer ran no browser, backend or CI. Behavioral PASS is attributed to
the supplied Chromium151 / Go1.27.2 / PostgreSQL18.6 log and inspected assertions;
the verdict certifies only the three scored fixes. Launcher/detector limitations
remain disclosed. Only this review file was written.
