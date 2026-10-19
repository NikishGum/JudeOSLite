## verdict

All four affected captures are valid: each was opened, shows its named content at the stated dimensions, and matches the updated screenshots.json hash.

| Original finding | Score | Evidence |
|---|---|---|
| 1. Family data remains after partial access revocation | resolved | `frontend/src/features/club-workspace/api-state.tsx` now revalidates on focus, visible-state changes and a 30-second visible timer, aborts older reads, preserves same-path data during background reads, and clears denied/gone data. `Families.tsx` renders the open parent member only while it remains in accessible families. The supplied Chromium/WebKit harness includes same-club two-family revocation with unchanged auth role keys: family A and its member dialog disappear while family B remains. It also checks that a nested account draft survives successful focus refresh. |
| 2. Pointer activation leaves square red tab outlines | resolved | Refreshed `plans-1440.png` and `finance-report-1440.png` retain the selected red label/underline without the former rectangle. `TabsTrigger` tracks pointer activation and clears it on blur/keyboard input; the CSS suppresses only pointer-focused tab outlines. Both engine runs include immediate pointer-outline assertions and an ArrowLeft keyboard-focus assertion. |
| 3. Active Documents tab disappears on phone | resolved | `member-Документы-390.png` and `member-Документы-320.png` visibly show the complete active «Документы» label and underline. `TabsList` locally scrolls the selected trigger into view after selection and resize; the supplied harness checks its bounds at 1440/390/320 and page-overflow limits. |

No regression attributable to this correction batch was found in the scoped evidence. Both supplied contract-verdict logs report PASS for 22 groups, and both result files contain zero page errors.

## remaining

Clear for the three scored fixes only. This verdict does not recertify the whole app, backend contracts, online payments, physical installation or staff acceptance. No browser or second detector was run. The original reference-image comparison limits remain. The still-running full PWA suite is not certified as passing by this verdict.

disposition: ship
