disposition: fix

## disposition

Fix the three frontend findings below in one batch, then return the affected captures and behavior evidence for a verdict pass. No rebuild or recapture is required for the current evidence: all 20 named images were opened, match their recorded hashes, have the stated viewport dimensions, and show their named content without blank or unloaded regions. Dialog and scrolled attendance captures are valid viewport captures under the packet.

## fidelity/direction

Review authority is the user-pinned, code-led direction contract; the original chat images are unavailable for pixel comparison.

| Element | Result | Evidence |
|---|---|---|
| TYPE | match | Rubik, restrained heading hierarchy, readable Russian names and tabular values across the desktop and phone captures. |
| MATERIAL | match | White panels, thin dividers, soft offset shadows and consistent vector icons; no imitation physical material or replacement raster assets. |
| GROUND | match | Cool light field specified by OWN-WORLD; implementation uses `#f2f6f9` in `frontend/src/app/app.css:6`. |
| Desktop shell | match | 248px Sidebar, 80px header, crimson active destination, original PRIDE emblem, context, notifications and profile in `home-1440.png` and the module captures. |
| Home composition | match | Four API-backed metrics, attendance overview and upcoming-workout column. Compact metrics on phone are an acceptable responsive adaptation to the Operate surface. |
| Families/member cards | adaptation with defect | The retained full native dialogs preserve task focus and existing forms, as explicitly required by the packet. The phone member tab strip loses the active destination; finding 3. |
| New sections | match at frontend scope | Families, child tabs/documents, payments/subscriptions/plans, notifications, financial reports, settings and parent views are present. Unavailable-module UI makes the missing backend visible instead of simulating success. |
| FORM/behavior | contradicted in part | Sidebar timing/no-wrap and selected calendar states have supplied checks; pointer/programmatic focus still leaves the square red rectangle on new tabs, finding 2. |

PRODUCT.md exists. Its older capability descriptions and DESIGN.md are explicitly awaiting the documenter under the authorized redesign; that sequencing is not a finding here. The user-pinned screenshot seed is recorded in FORM, so no open concept-roll requirement applies.

## material findings

1. **P2 — Revalidate family-scoped data when access changes without a club-role change.** `frontend/src/features/club-workspace/api-state.tsx:24` fetches only for the dependencies at line 61; `Families.tsx:47` retains `/api/portal/families` indefinitely. The app refreshes `/auth/me` on focus/visibility and every 30 seconds (`frontend/src/app/App.tsx:267`), but remounts the workspace by club-role keys (`App.tsx:720`). If a parent loses access to family A while retaining family B in the same club, those keys can remain unchanged and A's child/contact data and an already-open member profile stay displayed. This violates STORY's own-family scope and the preserved access-refresh lifecycle. Add guarded resource revalidation/invalidation on the existing lifecycle, cancel stale replies, and clear revoked entities/dialogs on denial or removal. Verify the same-club, two-family revocation case; backend denial of future reads remains #201's responsibility.
2. **P2 — The reported pointer-focus rectangle remains on the new Radix tabs.** In `plans-1440.png`, a square red outline surrounds «Тарифы» at approximately x481–558/y382–440; `finance-report-1440.png` shows it around «Финансы» at x407–493/y203–261. The supplied capture harness activates both with `.click()` (`frontend/e2e/screenshot_redesign.py:123`, line 126), before its later keyboard-focus test. The global selectors in `frontend/src/app/app.css:16`–18 and the wrapper in `frontend/src/components/ui/tabs.tsx:20` do not prevent this path. Make pointer/internal programmatic activation free of the unwanted rectangular ring while retaining a visible keyboard focus indicator; add the missing pointer assertion for these tab controls. Do not remove keyboard focus globally.
3. **P2 — Keep the active member tab visible on phone and after resizing.** `member-Документы-390.png` shows document content while its selected «Документы» tab and active underline are entirely offscreen; even «Платежи» is clipped at the right edge. `frontend/src/app/app.css:34` makes this a nonwrapping scroll strip, but `frontend/src/features/club-workspace/MemberProfile.tsx:63` has no visibility handling for the selected trigger. Preserve the selected tab in view on activation and viewport changes, or use a responsive arrangement that keeps the selected label visible. Confirm the same Documents state at390 and320 without page overflow.

## craft/behavior evidence

The manual craft-floor pass found the requested cool surfaces, readable type steps, consistent icons, restrained depth and useful unavailable/error/retry states in the reviewed evidence. The pinned app-metric/panel composition is appropriate to this brief. No decorative gradients, faux material, glyph substitutes or new shipping rasters were found in the sampled code/captures.

The provided Chromium and WebKit contract results each report 22 screen/state groups and zero page errors. The harness covers money-minor conversion and stable retry request IDs, document request/download URLs, read-all, attendance progression, and journal completion. Code plus the supplied checks support the trainer-selector removal, green attendance CTA state, zero-unmarked completion wording, phone-only settings treatment, calendar selection/date-arrow alignment, and non-squashing Sidebar transition. The focused-heading test covers programmatic headings and keyboard visibility; it does not clear finding 2's pointer-tab path. Static screenshots do not establish frame performance.

Keep the original logo/Rubik, clean shell geometry, explicit module-unavailable states, and existing complete native forms while applying these fixes.

## limits

No browser, server, detector or additional visual hunt was run by this reviewer. The detector attempt in the packet failed with127; the craft-floor check here is manual. This is a sampled frontend review, not an exhaustive read of every implementation path, and it certifies direction-contract fidelity rather than pixel fidelity to inaccessible attachments.

New module evidence uses explicitly synthetic API fixtures. Backend #201–#204, binary document delivery, actual iPay connection, real online payments, physical PWA installation and staff acceptance #188 are outside this verdict. Supplied completed legacy logs cover Chromium PWA/training/reports/dashboard and WebKit reports/dashboard; the remaining PWA/training harness runs identified as in progress are not certified as passing here.
