# Impeccable verdict pass · #214

Disposition: **ship — F1–F3 resolved**. This pass scores only the three findings from `finish-review.md`; it is not a new whole-surface review or CI/backend acceptance.

## 1. Request / scope fit

The correction batch addresses the requested focus handoff, date-list preview validity and mobile unavailable-state recovery within the Groups feature. No additional design work is requested by this verdict.

## 2. Evidence inspected

Read the final `GroupRoster.tsx`, `GroupScheduleEditor.tsx`, relevant `GroupManagement.tsx` and local CSS, `review-fix-checks.json`, `targeted-pass.log`, and the focused assertions in `frontend/e2e/groups_workspace.py`. Opened all seven regenerated roster/schedule/rule-editor/list captures, plus `adding-1440.png`, `adding-390.png` and `unavailable-390.png`. Captures are valid for their stated states. Full-page captures from document top establish layout; the focused measurements/log establish the scroll and focus behavior.

## 3. Fix scoring

| Finding | Score | Evidence |
| --- | --- | --- |
| F1 — adding opened offscreen | **Resolved** | `GroupRoster.tsx:24–34`, `:105` focus and reveal the adding heading, restore the prior trigger on close, and refocus the existing editor on repeated opening. `groups.css:28` provides a 100px scroll margin. Focused evidence reports heading Y=222.06 at1440px and124.17 at390px, with opened focus true; the test asserts return to the Add participants trigger. Both new adding captures show the complete workflow. |
| F2 — invalid future date-list picker | **Resolved** | `GroupScheduleEditor.tsx:19` initializes the picker at the later of today/effective start; `:31` advances it with the effective date. The focused test asserts native form validity initially and after advancing that date, then obtains a real preview; `targeted-pass.log` records PASS. |
| F3 — unavailable mobile detail trapped | **Resolved** | `GroupManagement.tsx:73` renders back-to-list for every selected detail independently of successful lookup. `unavailable-390.png` shows the visible recovery action; focused evidence records its successful return to the list. |

No finding is partial or unresolved.

## 4. Limitations

The reviewer did not execute a browser or backend. Behavioral evidence is the supplied focused log/measurements and inspected test assertions, supported by source and captures. The unrelated mandatory-CI failure is being investigated separately and is not passed or waived here. Launcher/detector did not run.

## 5. Disposition

**ship**, at the limited scope of the three scored fixes. The reviewer scored F1, F2 and F3 resolved; this statement does not certify unrelated checks or the entire application.
