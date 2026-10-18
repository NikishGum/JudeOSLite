## verdict

1. Resolved — the same eight participant detail/editor recaptures at 1280/900/390/320 now show an immediate top-right close control, with no title/control collision or horizontal clipping. The shared Dialog source gives it a 44×44px target, an accessible Russian name, the existing onCancel callback, and busy-state disabling; its header is sticky. The existing modal lifecycle, focus containment/restoration, and bottom dismissal remain. The supplied harness asserts full visibility before and after scrolling at all four widths. No material regression from this fix is visible in the recaptures. This ship verdict covers the scored fix, not the whole surface.

## remaining

clear

disposition: ship
