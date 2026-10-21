## verdict

Verdict pass, 21.10.2026: scored the five findings from the initial finish review only. Reopened all 20 captures at the same `/tmp/stats221-chromium/` and `/tmp/stats221-webkit/` paths: four paired reference/implementation captures, five shell content widths (319/320/390/768/1280), and the 1440px shell capture for each engine. Evidence is valid. No browser, test suite or detector was run by this reviewer.

| Finding | Score | Evidence |
| --- | --- | --- |
| 1. Local report ground | resolved | Sampled reference and implementation ground pixels now equal RGB 245,247,250 (#f5f7fa) in both engines, desktop and mobile. The shell retains its own background. |
| 2. Compact applied range | resolved | Both engines' paired and shell captures display `1–20 октября 2026`. Source preserves a single-date label and explicit full dates across month/year boundaries. |
| 3. Exact 80% explanation | resolved | Captures show `80% — ориентир этого макета, не норматив клуба.`; the extra desktop line is removed and the right-column density is restored. |
| 4. Duplicate group identity | resolved | Source inspection confirms expanded group buttons and journal headings use `groupLabel`, conditional on matching name/branch/archive status. `shortId` supplies an eight-character suffix and falls back to the full UUID on suffix collision; filter options use the same helper. Standard captures retain unsuffixed unique names. Duplicate-state captures were not supplied; this finding's resolution is based on the shared helper and its call sites, not an asserted visual test. |
| 5. Monochrome disclosure markers | resolved | WebKit mobile and shell captures now show quiet monochrome triangles in both disclosures, without orange markers. Source retains native details/summary semantics, open-state rotation, focus styling and 44px targets. |

No material regression introduced by this batch was observed within the scored scope. The necessary additional-metrics disclosure remains the previously accepted mobile adaptation.

## remaining

Clear for the five scored fixes. This verdict covers those fixes, not a new whole-surface review or independent verification of the parent-reported functional tests. No detector verdict is claimed; its launcher was unavailable. Extra duplicate/archive/long-name assertions were still running in the supplied status and are not represented here as PASS.

disposition: ship
