# Design QA

## Evidence

- Source visual truth: `/Users/chenping/.codex/generated_images/01a06652-fe87-7443-a47a-a117b6f794b4/exec-a617dd9c-bc87-4c1e-83cc-d9fa7615fe58.png`
- Implementation screenshot: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-implementation.png`
- Normalized comparison: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-comparison.png`
- Source pixels: 1408 x 1117
- Implementation capture pixels: 1408 x 792
- CSS viewport: 1408 x 1120 at device pixel ratio 1
- Normalization: the source was cropped to its top 1408 x 792 region and placed beside the implementation capture without scaling.
- State: dark theme, HTTP/HTTPS proxy page, new-rule modal open, default form values.

## Findings

- No actionable P0, P1, or P2 differences remain in the compared region.
- Fonts and typography: the implementation uses the existing macOS system-font stack and matches the reference hierarchy for modal title, section title, description, labels, controls, and help text.
- Spacing and layout rhythm: the modal frame, header spacing, two-column field grid, control heights, and section rhythm match the selected direction. The section divider now begins after the description instead of sitting below the entire heading row.
- Colors and visual tokens: dark surfaces use neutral graphite values without a blue cast; green remains limited to focus, enabled state, title icon, and section accents.
- Image and icon fidelity: the title uses the existing Phosphor icon library rather than a custom-drawn asset. The reference contains no raster content requiring generation.
- Copy and content: labels, descriptions, defaults, and helper text match the selected design and existing product behavior. The removed `基本信息` heading remains absent.

## Interaction Verification

- Opened the HTTP/HTTPS proxy page and the new-rule modal.
- Enabled HTTPS and confirmed certificate fields appeared.
- Disabled HTTPS and confirmed conditional fields disappeared.
- Checked the browser console: no errors.

## Comparison History

- Before the final comparison, user feedback identified that section dividers were below the whole heading row. The bottom border was removed and replaced with a flexible line after the section description.
- The post-fix normalized comparison shows the divider in the intended trailing position with no remaining P0/P1/P2 issue.

## Follow-up Polish

- P3: browser surface height limited the normalized side-by-side evidence to the upper 792 px; lower sections retain the same shared section and field styles.

## Help Text Fidelity Check — 2026-09-04

- Source visual truth: `/var/folders/sp/90pk0ss17bj66n2wcc3fgx1h0000gn/T/codex-clipboard-5d960657-fe21-4f2b-b541-277f89ff1118.png`
- Implementation screenshot: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-helptext-implementation.png`
- Source pixels: 1300 x 178
- Implementation capture pixels: 1280 x 720
- CSS viewport and density: 1280 x 720 at device pixel ratio 1
- State: dark theme, HTTP/HTTPS proxy page, new-rule modal open.
- Full-view evidence: the restored helper appears directly below the domain textarea without shifting the surrounding entry fields.
- Focused comparison evidence: the supplied focused crop and the browser-rendered modal were presented together; the wording, muted color, single-line treatment, and textarea-to-helper spacing match the requested reference.
- Fonts and typography: unchanged existing system-font stack, size, weight, line height, and antialiasing.
- Spacing and layout rhythm: unchanged; only the helper copy was replaced.
- Colors and visual tokens: unchanged muted helper-text token.
- Image quality and asset fidelity: no image assets are involved in this text-only adjustment.
- Copy and content: restored to `多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。`
- Interaction verification: opened the proxy page and new-rule modal; exact helper text is visible.
- Comparison history: the abbreviated copy was replaced with the full source wording; the post-fix comparison found no P0, P1, or P2 differences in the requested region.

## Target Host Width Check — 2026-09-04

- Source visual truth: `/var/folders/sp/90pk0ss17bj66n2wcc3fgx1h0000gn/T/codex-clipboard-f41a340f-a7ea-4dc8-87da-f2226c96015d.png`
- Implementation screenshot: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-target-host-width.png`
- Source pixels: 1768 x 464
- Implementation capture pixels: 1280 x 720
- CSS viewport and density: 1280 x 720 at device pixel ratio 1
- State: dark theme, HTTP/HTTPS proxy page, new-rule modal open, single backend service selected.
- Full-view evidence: the backend-service section retains its existing two-column grid and surrounding section rhythm.
- Focused comparison evidence: the supplied backend-service crop and browser-rendered modal were presented together; the target-host input now uses the same content width as the backend-service-pool select above it.
- Fonts and typography: unchanged.
- Spacing and layout rhythm: left edges and right edges of the two left-column controls align; labels and row gaps are unchanged.
- Colors and visual tokens: unchanged.
- Image quality and asset fidelity: no image assets are involved.
- Copy and content: unchanged.
- Interaction verification: opened the proxy page and new-rule modal; the single-backend-service fields render normally.
- Comparison history: the target-host input previously inherited the 50% text-input width; a scoped override changed only this input to 100%, with no remaining P0, P1, or P2 issue in the requested region.

final result: passed

## Smoked Glass Button System — 2026-09-04

### Evidence

- Source visual truth: `/Users/chenping/.codex/generated_images/01a06b44-3cbd-7b91-8e4f-f472bcfb351d/exec-92a3136b-45e1-431a-ad6c-87921c5f9da6.png`
- Browser-rendered implementation: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-button-style-3-implementation.png`
- Normalized source: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-button-style-3-source.png`
- Full-view comparison: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-button-style-3-comparison.png`
- Focused button comparison: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-button-style-3-focused-comparison.png`
- Narrow-screen evidence: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-button-style-3-narrow.png`
- Source pixels: 1672 x 941; normalized to 1440 x 810 with no crop.
- Implementation pixels and CSS viewport: 1440 x 810 at device pixel ratio 1.
- State: dark theme, TCP/UDP proxy empty state, complete local management service connected through its Unix socket.

### Findings

- No actionable P0, P1, or P2 differences remain for the selected button direction.
- Fonts and typography: the existing system-font stack, compact 12 px button labels, weights, and line heights remain intact. Icons use the installed Phosphor set and align optically with the labels.
- Spacing and layout rhythm: primary toolbar buttons render at 38 px high with 9 px radii, 7 px icon gaps, close shadows, and the existing toolbar spacing. The generated source's altered content proportions were not copied because the request scoped the change to the button system.
- Colors and visual tokens: dark buttons use translucent graphite centers, cool-silver borders, restrained inner highlights, and a muted cobalt icon accent. The former mint dark-theme accent is replaced by slate blue; offline and destructive states remain semantic red.
- Image and icon fidelity: all action glyphs use the existing Phosphor icon library. No raster placeholders, custom SVGs, CSS drawings, or generated icon assets were introduced.
- Copy and content: all visible action labels remain unchanged; decorative full-width plus characters were replaced by accessible leading `PlusCircle` icons.
- Responsive behavior: at 720 x 900, toolbar actions wrap without horizontal overflow; document and body scroll widths both equal the 720 px viewport.

### Interaction Verification

- Opened the TCP/UDP page and activated `添加 TCP/UDP 规则`; the complete rule modal opened.
- Closed the modal and used keyboard navigation; the focused navigation button showed the two-ring slate-blue focus treatment.
- Verified the primary action's computed geometry and paint: 38 px height, 9 px radius, translucent graphite background, cool-silver border, and compact layered shadow.
- Checked the browser console after navigation and modal interaction; no errors or warnings were reported.

### Comparison History

- First normalized full-view and focused comparisons found no actionable P0, P1, or P2 mismatch within the button-only scope, so no visual correction loop was required.

### Follow-up Polish

- P3: hover and pressed depth can be tuned further after subjective review, but both states are implemented and keyboard focus is visibly distinct.

final result: passed

## Hand-drawn Sidebar Icon Refinement — 2026-09-04

- Source visual truth: `/Users/chenping/.codex/generated_images/01a06b45-8b4a-7452-9e72-3fd9d1d0c27c/exec-0fbbc702-e0ad-4581-90b3-bf58e85a8e8d.png`
- Browser-rendered implementation: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-handdrawn.png`
- Focused implementation crop: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-handdrawn-crop.png`
- Normalized comparison: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-handdrawn-comparison.png`
- Source pixels: 825 x 1907.
- Implementation pixels: 1440 x 1024, CSS viewport 1440 x 1024 at device pixel ratio 1.
- Focused comparison: source scaled proportionally to 220 x 509 and padded to 220 x 520; implementation cropped to the same 220 x 520 sidebar region.
- State: dark theme, `Nginx 配置` selected, complete local management service connected.

### Findings

- No actionable P0, P1, or P2 differences remain for the user-requested hand-drawn icon refinement.
- Fonts and typography: unchanged from the existing product UI.
- Spacing and layout rhythm: every SVG measures 20 x 20 CSS pixels in the browser and remains optically centered in the existing navigation slot.
- Colors and visual tokens: all paths use `currentColor`; inactive, hover, focus, and selected colors continue to come from the existing navigation states.
- Image and icon fidelity: the ten icons now use one purpose-built 1.75 px monoline system with matching rounded caps and joins. The hand-drawn variant recreates the reference's four-cell overview, paired HTTP arrows, five-node stream topology, stacked server pool, gauge, certificate seal, terminal, revision clock, code brackets, and sliders. This custom SVG treatment is intentional because the user explicitly requested manual drawing after rejecting the closest library matches.
- Copy and content: unchanged.

### Interaction Verification

- Selected `Nginx 配置` through the sidebar and confirmed the custom code icon inherits the mint active color.
- Confirmed all ten icons remain visible at the compact production sidebar density.
- The complete-service page reported no console errors or warnings for `127.0.0.1:4174`.

### Comparison History

- The first library-based implementation retained noticeable shape differences.
- Replaced the navigation icon mapping with a dedicated SVG component, then enlarged the internal drawing viewport and redrew the slider tracks so the final 20 px rendering more closely matches the source.
- The post-fix normalized comparison contains no remaining actionable P0, P1, or P2 issue.

### Follow-up Polish

- P3: the generated source uses an enlarged presentation scale and omits the real product brand/footer; the implementation intentionally retains the established 220 px production sidebar.

final result: passed

## Sidebar Icon Direction 1 — 2026-09-04

- Source visual truth: `/Users/chenping/.codex/generated_images/01a06b45-8b4a-7452-9e72-3fd9d1d0c27c/exec-0fbbc702-e0ad-4581-90b3-bf58e85a8e8d.png`
- Browser-rendered implementation: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-implementation.png`
- Focused implementation crop: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-crop.png`
- Normalized comparison: `/Users/chenping/Project/codex/fnos/fn-nginx-web/design-qa-sidebar-icons-comparison.png`
- Source pixels: 825 x 1907.
- Implementation pixels: 1440 x 1024, CSS viewport 1440 x 1024 at device pixel ratio 1.
- Focused comparison dimensions: source scaled proportionally to 220 x 509 and padded to 220 x 520; implementation sidebar cropped to 220 x 520; combined evidence is 440 x 520.
- State: dark theme, `Nginx 配置` selected, complete local Go management service connected through its Unix socket.

### Findings

- No actionable P0, P1, or P2 differences remain for the requested icon-only change.
- Fonts and typography: the existing product font stack, label sizes, weights, and line heights were intentionally preserved.
- Spacing and layout rhythm: the existing 20 px icon slot, row height, padding, chevrons, brand block, and footer were preserved. The generated concept exaggerates the sidebar scale, so the implementation correctly follows the existing product density instead of copying that artifact.
- Colors and visual tokens: the dark neutral background, muted blue-gray inactive color, mint active color, and translucent green selected row remain unchanged.
- Image and icon fidelity: standard UI icons use the existing Phosphor library. Direction 1 is represented by `SquaresFour`, `ArrowsLeftRight`, `ShareNetwork`, `HardDrives`, `Gauge`, `Certificate`, `TerminalWindow`, `ClockCounterClockwise`, `Code`, and `SlidersHorizontal`; no custom SVG, CSS drawing, raster placeholder, or new dependency was introduced.
- Copy and content: all ten navigation labels and ordering are unchanged.

### Interaction Verification

- Opened the full local management service at a desktop viewport and selected `Nginx 配置`.
- Confirmed the selected row, generated-config page, sidebar footer status, and all ten navigation icons render together.
- Console entries for the full-service URL contained no errors or warnings. Earlier errors were isolated to the API-less Vite-only preview and are not present in the complete app runtime.

### Comparison History

- First normalized side-by-side comparison found no actionable P0, P1, or P2 issue. No visual correction loop was required.

### Follow-up Polish

- P3: the generated concept uses a larger presentation scale than the production sidebar; the implementation intentionally keeps the product's established compact 220 px sidebar.

final result: passed

## Current Build Gate — Smoked Glass Buttons

- Latest evaluated change: smoked-glass button system.
- Full and focused evidence: `design-qa-button-style-3-comparison.png` and `design-qa-button-style-3-focused-comparison.png`.
- Browser interaction, keyboard focus, console, desktop viewport, and 720 px responsive checks passed.

final result: passed
