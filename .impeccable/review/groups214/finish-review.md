# Independent Impeccable finish review · #214

Disposition: **fix**. Reviewed 20 October 2026 against `/tmp/issue214.md`, the Groups direction, PRODUCT/DESIGN, design-system guidance, current Groups source and all seven supplied fresh captures. No application files edited by the reviewer.

## 1. Request / scope fit

The implementation follows the requested Operate extension: desktop list/detail, mobile list → detail, branch identity, roster and schedule tabs, current/future rules, bounded real sessions and contextual exceptions. PRIDE, Rubik and incumbent surfaces remain recognizable. Diff against `9ca620d` contains no Sessions, shared editor, route, global CSS/token, API or backend changes. Source preserves existing request paths, versions, preview tokens and guarded navigation. This is a source/screenshot assessment, not backend acceptance.

## 2. Visual / interaction evidence

Opened `roster-1440.png`, `roster-390.png`, `schedule-1440.png`, `schedule-390.png`, `rule-editor-1440.png`, `rule-editor-390.png`, `list-390.png`, plus incumbent `baseline.png`. All named captures show the expected loaded surface from document top; none requires recapture. Mobile fixed navigation is at the captured viewport bottom, with remaining content scrollable. Long group names wrap; selected group/tab, branch, trainer, dated future changes, cancellation reason and single-date exceptions remain readable. Local tabs have keyboard handling and labels; actions and fields use native semantics. No comp or new shipping imagery applies to this ordinary extension.

## 3. Actionable material findings

| ID | Priority | Evidence | Required correction |
| --- | --- | --- | --- |
| F1 | P2 | `frontend/src/features/groups/GroupRoster.tsx:87`, `:95`: opening adding only sets state; the editor is appended after the entire roster and has no focus/scroll handoff. The mobile roster capture already places its second member well below the CTA. | Move focus to the opened adding heading/search and bring it into view, with a usable return to its trigger on close. On an ordinary roster, clicking the main CTA must reveal the workflow without manually searching past members. |
| F2 | P2 | `frontend/src/features/groups/GroupScheduleEditor.tsx:12–19`, `:64`: a future rule initializes effective `from` to the day after its start, but `picked` to today. The required date input then has `value < min`; native constraint validation prevents the whole preview form submitting even when its selected dates are valid. Advancing the effective date can cause the same problem. | Keep the temporary date picker valid as effective date changes, or isolate/validate it when adding a date so it cannot block preview of an otherwise valid date-list rule. Verify editing a future date-list rule and moving its effective date forward. |
| F3 | P2 | `frontend/src/features/groups/GroupManagement.tsx:74`, `:80`; `groups.css:63–67`: the back control exists only inside the successful group branch. With an unknown/inaccessible detail ID, mobile CSS hides the list and creation heading, leaving “выберите другую группу” with no control to do so. | Render the back-to-list action for selected mobile detail state independently of successful group lookup, including unavailable/error recovery. |

These are the bounded fix list; no aesthetic rebuild or optional polish requested.

## 4. Limitations

No browser, keyboard, network or backend execution was performed by this reviewer. The parent reports targeted API/PostgreSQL/browser checks; extended checks and CI were pending in the review packet and are not certified here. Captures do not demonstrate the open multi-add/partial/unknown-result UI or preview confirmation/error states; those paths were assessed in source. Craft-floor reference became available to this reviewer and was read directly; launcher and detector did not run. Manual review cannot certify computed contrast, complete focus behavior or all device states.

## 5. Disposition

**fix** — apply F1–F3 together, then return the same seven viewport/state captures and focused evidence for the three fixes for a verdict pass. Preserve the Sessions boundary. No recapture or rebuild directive applies to the evidence reviewed here.
