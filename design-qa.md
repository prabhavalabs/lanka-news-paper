# Login design QA

- Source visual truth: `/var/folders/4c/_xd1rpdd3w1cxmr2qscmyxkr0000gn/T/codex-clipboard-e1c5dd77-d186-46eb-a964-42f05a16daef.png`
- Implementation screenshot: `/Users/nipuntheekshana/.codex/visualizations/2026/08/16/01a00ba8-64ab-7820-af44-c5f7dde77079/admin-login-no-theme-toggle-1794x1046.png`
- Combined comparison: `/Users/nipuntheekshana/.codex/visualizations/2026/08/16/01a00ba8-64ab-7820-af44-c5f7dde77079/admin-login-theme-removal-comparison-matched.png`
- Viewport: 1794 × 1046 CSS px
- Density normalization: source 3588 × 2092 px treated as a 2× capture and downsampled to 1794 × 1046; implementation captured at 1794 × 1046 with device scale factor 1
- State: signed out, dark theme

## Full-view comparison

The reference identifies the theme switcher in the upper-right corner. The updated implementation removes that control from the login route while preserving the centered login card and all form content. The theme provider remains active, so the page still follows the selected or system theme without exposing a login-page switcher.

Focused-region comparison was not needed because the requested change is isolated in a large, empty corner and is unambiguous in the matched full-view comparison.

## Required fidelity surfaces

- Fonts and typography: unchanged.
- Spacing and layout rhythm: unchanged; removing the absolutely positioned switcher does not affect the centered card.
- Colors and visual tokens: unchanged; the active theme still supplies the page tokens.
- Image quality and asset fidelity: no image assets exist in this view.
- Copy and content: login copy and fields are unchanged.

## Comparison history

1. Previous P2 — the first login implementation used an oversized card and controls.
   - Fix: normalized the card, padding, controls, typography, radii, and spacing.
   - Post-fix evidence: `admin-login-dark-827x624-final.png`.
2. Current request — the theme switcher was visible on the login page.
   - Fix: removed only the login-page import and rendered control.
   - Post-fix evidence: `admin-login-no-theme-toggle-1794x1046.png` and `admin-login-theme-removal-comparison-matched.png`.

## Interaction and responsive checks

- Browser query confirms zero System, Light, or Dark theme buttons on the login route.
- Login heading and form remain rendered.
- Browser console errors: none.
- Admin production build and TypeScript compilation pass.

## Findings

No actionable P0, P1, or P2 differences remain for the requested removal.

final result: passed

---

# Dashboard design QA

- Current-state reference: `/var/folders/4c/_xd1rpdd3w1cxmr2qscmyxkr0000gn/T/codex-clipboard-5e913106-f811-4ef6-aa39-f49af2acac63.png`
- ShadCN target reference: `/var/folders/4c/_xd1rpdd3w1cxmr2qscmyxkr0000gn/T/codex-clipboard-277bf501-129f-4b23-b3fa-eca47c288124.png`
- Implementation: `http://127.0.0.1:5174/`
- Captures checked: 1600 × 900 wide dark, 1280 × 720 desktop dark, 1280 × 720 desktop light, and 390 × 844 mobile dark

## Side-by-side comparison

The implementation matches the reference hierarchy: inset collapsible sidebar, compact sticky header, four metric cards, a large interactive area chart, and an operational table. The generic ShadCN demo labels and fake revenue data were replaced with newsroom navigation, live publishing metrics, source health, editorial activity, and the real review queue.

The wide layout renders four equal cards in one row and the main chart beneath them. At the normal desktop width the cards form a balanced two-column grid, and on mobile they stack without horizontal page overflow. The typography, rounded surfaces, subtle gradients, blue primary action, quiet borders, and restrained shadows follow the installed preset in both light and dark themes.

## Interaction and responsive checks

- System theme is the default; light and dark modes both render correctly.
- Trend controls load real 7, 30, and 90-day API ranges; mobile defaults to 7 days.
- Queue search, status filter, column visibility menu, and pagination controls render and respond correctly.
- Sidebar collapses out of the mobile viewport; mobile document width remains within the viewport.
- One page-level `h1`; all icon-only controls have accessible names.
- Fresh browser load reports no console warnings or errors.

## QA fixes

1. Linked buttons initially emitted Base UI accessibility warnings.
   - Fix: declared non-native button rendering for link-backed actions.
2. The mobile columns trigger had no accessible name.
   - Fix: added an explicit control label.
3. Opening the column menu initially failed because its label lacked a menu group.
   - Fix: wrapped the label and checkbox items in the required group and retested column hiding.

## Verification

- `pnpm --filter @snap/admin build`
- `go test -race ./...`
- `go vet ./...`
- Wide, desktop, light-theme, mobile, interaction, and clean-console browser checks

No actionable P0, P1, or P2 differences remain for this dashboard revamp.

final result: passed
