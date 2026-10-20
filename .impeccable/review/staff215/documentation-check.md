# UX-STAFF #215 — documentation check

20.10.2026. Ordinary extension of the incumbent PRIDE runtime; selected variant Б
«Карточка доступа». This is a documentation/source comparison, not a new visual
direction or an independent execution of the browser/backend checks.

## Outcome

No new durable global token, identity, font, asset family or component philosophy
is required. Preserve `DESIGN.md` and `.impeccable/design.json` as they stand.
The employee roster/access card, its local responsive composition and mobile
filter disclosure belong to this surface contract, not to a new global world.
No root design file was written, no init/repair/drift workflow was run, and no
overwrite approval was needed or requested.

## Evidence compared

- `PRODUCT.md`, all of `DESIGN.md`, `.impeccable/design.json` and
  `design-system/README.md`; project rules and the staff entries in the knowledge
  base; Impeccable `reference/document.md` under the ordinary-extension constraint.
- `.impeccable/surfaces/frontend-src-features-staff.md`, all of
  `frontend/src/features/staff/StaffAccess.tsx`, `StaffEditor.tsx` and
  `staff-access.css`; the `frontend/src/app/App.tsx` diff and incumbent CSS rules
  in `frontend/src/app/app.css`.
- `docs/staff-access.md`, `docs/staff-access-validation.md`,
  `.impeccable/review/staff215/finish-review.md`, the final Chromium/WebKit logs,
  Chromium confirmation logs and `detector.log`; the browser scenario's explicit
  synthetic-data/disposable-PostgreSQL setup.
- Follow-up comparison: the completed candidate lookup in
  `backend/internal/auth/staff_directory.go`, route/middleware wiring, related
  `staff_directory_test.go` cases, the revised `StaffEditor.tsx` and assignment
  call, revised `docs/staff-access.md`, `staff-api-final.log`,
  `go-checks-final.log`, `chromium-candidates.log`, the subsequently completed
  `chromium-existing-verdict.log` and `webkit-confirmation.log`, plus the current
  `finish-verdict.md` and shipping-image build log.

| Incumbent system | Staff extension evidence | Documentation consequence |
| --- | --- | --- |
| Rubik, PRIDE shell, semantic red/neutral theme | Shared shell/avatar/dialog; semantic `--card`, `--foreground`, `--primary`, `--muted`, `--secondary` references | Preserve the existing identity and type system. |
| White panels, 16px corners, `0 4px 18px rgb(32 51 75 / 4%)` shadow | Same panel values as `app.css:40` | No new material/elevation token. |
| Staff rows with 16px vertical / 12px horizontal padding; readable long values | Matching padding and `overflow-wrap: anywhere` | Existing row/readability rules remain sufficient. |
| Text-labelled success/warning states and shared field/button rules | Existing `--panel-success*`, `--panel-warning*`, radius tokens and shared controls | No new status palette or control language. |
| Responsive content varies by task; keyboard focus and explicit exits | Local 1100px list/card switch, 760px filter disclosure, mobile return and shared guarded sheet | Keep these composition/interaction details surface-local. |

`staff-access.css` introduces no CSS custom-property declarations, font import or
image URL. The reviewed staff components introduce no raster imports. The existing
PRIDE logo and Rubik provenance remain with `design-system/manifest.json` and the
font license; synthetic screenshots are review evidence, not shipping assets.

At this check, `git diff --name-only -- DESIGN.md .impeccable/design.json design-system`
was empty. Root-file SHA-256 values were:

- `DESIGN.md`: `b05aff0c7ffb9a62079d6401151517ce47c70ce444ad6749b39f4b32209f4e4d`
- `.impeccable/design.json`: `23b8aa03f86eb2cb49cd92c4e50d80bb12c78498e09f44c1b559244d3894c56e`

## Task-document truth and remaining evidence

No factual contradiction was found in `docs/staff-access.md` within this review's
source scope. Its one-account/multiple-assignments model, role+club predicate,
recipient retention, account-versus-assignment consequences, explicit version
reload, nonsecret draft preservation and password clearing match the frontend.
No edit to that task document was necessary. This comparison does not independently
certify every backend authorization or transaction claim in the API sections.

The revised candidate contract also corresponds to the checked source:

- `GET /api/staff-candidates?query=…` is registered with owner-only middleware
  and shares the directory's `REPEATABLE READ READ ONLY` helper, including a
  second owner/session check inside its snapshot.
- The trimmed query must contain 3–100 Unicode characters. SQL parameterization
  and explicit escaping of `%`, `_` and backslash make the case-insensitive
  substring match literal; the result is bounded to 30 and returns `StaffCard[]`.
  The form asks the owner to refine a query returning 30 results.
- The candidate path can read non-owner parent-only identities without adding
  them to the staff directory. Its fields contain account/assignment information,
  with no family IDs, relationships or family records. An explicit selected
  candidate ID and version feed the existing role-assignment operation; no
  duplicate account/create request is used for that path.
- Source and integration assertions preserve the selected login and
  `family_access` through parent→staff+parent assignment. Candidate lookup alone
  leaves the directory unchanged. The frontend aborts old lookups on query/method
  changes and unmount; it clears the candidate choice when the query changes.
- The method option now reads «Существующий аккаунт». The selected full name and
  login are repeated outside the native select in an `aria-live` text block;
  existing dialog `overflow-wrap: anywhere` covers that block. This uses incumbent
  typography/controls and requires no global token. Browser confirmation and the
  independent visual re-score remain separate evidence gates.

Both bounded finish-review fixes are present in source: the phone-only disclosure
has `aria-expanded`, `aria-controls` and an active-filter count; the roster select's
visible, programmatically associated label is «Филиал сотрудника». They retain the
existing filter values/predicates. `finish-review.md` is the preceding
`disposition: fix` review. The checked `finish-verdict.md` resolves those original
two fixes, then identifies two bounded findings in the candidate form. Current
source contains their copy and wrapping-recipient corrections; that report still
awaits the independent re-score of the corrected evidence.

The initial `chromium-final.log` and `webkit-final.log` report PASS with empty
JavaScript-error lists. The later `chromium-confirmation.log` is a failed attempt
at the mobile `scrollY == 0` assertion; `chromium-confirmation2.log` reports PASS,
including the new collapsed/expanded/active-filter captures at 390px and 320px.
Those outcomes must remain distinguishable in the final validation record. The
original two-engine PASS logs precede the candidate addition and cannot certify
its final corrected form.

`staff-api-final.log` contains passing auth/clubs package results. The checked
candidate integration cases cover owner lookup, parent-only exclusion until
assignment, unchanged family access, a 30-result bound and literal wildcards.
`chromium-candidates.log` reports a complete candidate-enabled Chromium scenario
PASS with `jsErrors: []`; the scenario checks the account count and family link
after conversion. `go-checks-final.log` contains passing package results, but its
presence alone is not proof of the parent command's successful completion or
every subsequent build step. The corrected existing-account label/wrapping block
subsequently received a bounded Chromium PASS in `chromium-existing-verdict.log`
with empty JavaScript errors. `webkit-confirmation.log` also subsequently reports
PASS with `staff=10`, empty JavaScript errors and the existing-account, list,
filter, card and form capture set. `docker-build-shipping.log` reaches successful
image export. These are supplied execution logs, not checks independently run by
this documenter. The implementation owner was notified to distinguish the older
two-engine pass, candidate pass and corrected-form confirmation in the validation
record. The final independent visual re-score was still pending at this check;
CI success and main delivery are not certified here.

Launcher/detector absence is disclosed in the task documents and supported by
`detector.log`; detector PASS is not claimed. The direct source/document fallback
does not establish automated detector results. No physical-device, assistive-
technology, HTTPS/PWA-installation or employee-acceptance #188 result is added by
this check. Synthetic names and captures do not certify real club data or user
acceptance. Only this documentation-check file was written.
