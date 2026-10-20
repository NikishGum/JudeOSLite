disposition: fix

QUALITY BAR card and detector/hook findings were not supplied; launcher/context/detector were unavailable, so no detector PASS is claimed. Review used supplied captures, source and logs, without a browser or independent execution of the application.

## persistence

Pass. `PRODUCT.md`, `DESIGN.md`, `design-system/README.md`, `docs/staff-access.md` and `.impeccable/surfaces/frontend-src-features-staff.md` persist the product, incumbent world, API/operation truth and chosen direction. This is a code-led extension of the existing PRIDE world. Variant Б is explicitly selected in live issue #215; FORM records this selection and the exemption from rerolling a seed. Comp reproduction state/spec/diff gates are not applicable.

Evidence check passed before the visual verdict: all required Chromium captures exist, decode, contain the document top or the named form, have the claimed width, and contain valid content rather than blank/black regions. The full 12-capture WebKit parity set also passed this check. Full-page captures have their expected document heights; forms are viewport-height sheets. Fixed bottom navigation within a full-page capture is an expected capture behavior, not malformed evidence.

Opened and visually inspected all files in these sets (the braces enumerate the actual filenames):

- `.impeccable/review/staff215/round1/chromium-{list,card,form}-{1440,1280,390,320}.png` — all 12.
- `.impeccable/review/staff215/round1/webkit-{list,card,form}-{1440,1280,390,320}.png` — all 12.
- `/tmp/ux215-reference/design-system/prototypes/staff/evidence/{b-desktop,b-mobile,b-mobile-detail}.png` — structural critique references.
- `chromium-list-390-viewport-review.png` and `chromium-list-320-viewport-review.png` — review-only crops of the first 844 rows of the supplied full-page captures, confirming what the initial viewport contains; these are not new browser captures.

Source sampled: all of `StaffAccess.tsx`, `StaffEditor.tsx`, `staff-access.css`, `staff-types.ts`, and the exact `frontend/src/app/App.tsx` diff. Supplied `chromium-final.log` and `webkit-final.log` report PASS with `jsErrors: []` for the real browser → Go → PostgreSQL scenarios and all roles. `go-checks.log` and `staff-api.log` report passing package/integration checks, including snapshots, atomicity, stale/live scope and races. Those logs support the parent’s execution claims; this review does not independently certify the backend, physical devices, all assistive technologies or unrelated sections.

## fidelity

The prototype’s salient decisions are one searchable employee roster, a single selected account card, clear account actions above separate role/club assignments, and phone list → card navigation with an explicit return. It is a structural critique reference rather than a pixel comp; no comp element matrix or asset reproduction obligation is imposed.

| Contract check | Verdict | Evidence |
| --- | --- | --- |
| TYPE | match | Rubik, incumbent weights/scale, readable hierarchy; names and login wrap within the access card. `staff-access.css:24`, `:31`, `:79`–`:84`; desktop/card and both phone/card+form sizes. |
| MATERIAL | match | White working surfaces, fine internal dividers, subtle offset soft shadow, Lucide icons, no imitation physical material. `staff-access.css:7`, `:37`, `:44`; no new raster material is promised or needed. |
| GROUND | match | Dominant field is RGB(242,246,249) / `#f2f6f9`, confirmed from Chromium and WebKit pixels, matching DESIGN.md; panels are `#ffffff`. |
| THESIS | match | One account per row/card, textual account state distinct from assignments; explicit unassigned state. `StaffAccess.tsx:169`–`:190`, `staff-types.ts:7`. |
| OWN-WORLD | match | PRIDE shell, Rubik, cold ground, red actions, success/warning semantics remain intact. The App diff replaces only the former split staff presentation and its imports/state. |
| STORY | match within reviewed evidence | Search and role+club predicates retain the selected recipient; filtered-out selection is explicit, pending/version/network/reload states have recovery copy. `StaffAccess.tsx:53`–`:61`, `:87`–`:116`, `:178`, `:198`–`:202`; supplied browser logs support executed behavior. |
| FIRST VIEWPORT — desktop | match | Compact roster beside selected access card; primary add action above; identity → password/all-access → assignments reading order remains clear at 1440 and 1280. |
| FIRST VIEWPORT — phone | contradicted | At both 390×844 and 320×844, the default expanded three-filter stack consumes the remaining workspace; no complete employee row is visible before the fixed bottom navigation. The first selectable identity therefore requires scrolling before browsing can begin. See both `chromium-list-*-viewport-review.png` crops and WebKit phone/list parity captures; cause is the unconditional filters at `StaffAccess.tsx:161`–`:165` and single-column rule at `staff-access.css:78`. |
| FORM | match | Explicitly pinned variant Б, existing shell and token world; no new concept or brand choice was required. Forms preserve recipient identity, clear submission/cancellation, and the existing guarded sheet behavior. |

Truth/floor: fixture names are explicitly synthetic stress data, not production copy. No fabricated claims, invitation workflow, role switcher, A/B comparison panel, family details or fake saves appear in the reviewed UI. The very long selected name is intentionally preserved and wraps; its length is not a copy defect. No material craft-floor violation was found in the changed surface beyond the phone first-viewport task failure recorded above. WebKit’s backdrop rendering differs from Chromium, but the sheet remains separated and readable; DESIGN.md makes blur conditional on support.

## ceiling

The incumbent Operate world’s useful devices are present: calm two-column desktop composition, a distinct selected row, a restrained red action hierarchy, soft panel depth, readable assignment boundaries, and an explicit mobile return. The remaining useful device is progressive disclosure for secondary mobile filters, so the first viewport exposes actual staff. No ornamental, lettering or motion escalation is justified by this task. An independent QUALITY BAR-card ceiling comparison is unavailable because no card was supplied.

## material_fixes

1. **FIRST VIEWPORT / Operate usability:** At phone widths, keep search and result count visible and put role/employee-club/access filters behind a clearly labelled ≥44px disclosure (with an active-filter count or summary), or an equally compact treatment that exposes at least the first normal employee row above the fixed dock at both 390×844 and 320×844; retain all values, filtering semantics, list scroll/selection and return focus. Source: `StaffAccess.tsx:158`–`:166`, `staff-access.css:75`–`:79`. Evidence: both Chromium viewport-review crops, Chromium/WebKit phone/list captures. Verify expanded, collapsed and active-filter states in the bounded confirmation capture batch.
2. **STORY / context clarity, parent-reported compatibility finding:** Rename the roster filter label from «Филиал» to «Филиал сотрудника» (or another equally explicit scope label) so it is distinguishable from the global header’s «Филиал» context control; update exact-label consumers and confirm the visible/accessibility label in the same batch. Source before the fix: `StaffAccess.tsx:163`; current captures still show «Филиал». This distinction affects two controls with different consequences, rather than changing the filter predicate.

## keep

Preserve variant Б’s one-person roster/access-card model, the PRIDE/shadcn/Rubik shell, explicit account-versus-assignment consequences, protected form lifecycle, server-confirmed recipient/versions, and full long-name readability while making the two bounded fixes.
