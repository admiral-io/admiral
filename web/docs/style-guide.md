# Admiral Web Style Guide

Design guidelines for the Admiral frontend. The goal is a cohesive, professional identity that leverages MUI defaults while feeling distinctly "Admiral."

## Color Identity

**Primary: Admiral Navy** — authoritative, technical, infrastructure.
**Accent: Warm Amber/Gold** — action, warmth, contrast against navy.

Both light and dark modes share the same identity, adjusted for contrast:

| Token         | Light Mode | Dark Mode  | Usage                        |
|---------------|------------|------------|------------------------------|
| primaryMain   | `#1b2a4a`  | `#5b8def`  | Headers, key UI elements     |
| primaryLight  | `#e8edf5`  | `#93b8f9`  | Backgrounds, hover states    |
| primaryDark   | `#0f1b33`  | `#3d6bc7`  | Emphasis, pressed states     |
| secondaryMain | `#d97706`  | `#f59e0b`  | CTAs, active indicators      |
| secondaryLight| `#fef7e8`  | `#fde68a`  | Subtle highlights            |
| secondaryDark | `#92400e`  | `#d97706`  | Hover on amber elements      |

Color tokens live in `src/theme/colors.ts` as structured `light`/`dark` exports.

## Typography

- **Font:** Geist Variable (with Helvetica/Arial fallback)
- **Headings:** 600-800 weight, tight letter-spacing at larger sizes
- **Body:** 0.875rem (14px) base, 400 weight
- **Buttons:** 600 weight, no text-transform

## Component Philosophy

**Let MUI be MUI.** The theme defines identity (colors, border radius) and corrections (dark mode fixes, accessibility). It does not redefine how buttons, inputs, or tables fundamentally work.

Customizations we apply:
- Flat cards (elevation 0) with subtle borders instead of shadows
- 8px border radius on interactive elements (inputs, buttons, cards)
- Custom scrollbar styling
- Focus-visible outlines for accessibility
- 1.5px border width on outlined elements for crispness

## Page Patterns

### Error Pages (404, Auth Error, etc.)

Full-viewport, no card wrapper. The page is the error.

**Layout:**
```
[full viewport, subtle gradient background]

    404                          <- large, bold navy (~6rem)
    Page not found               <- h5, secondary text color
    Explanation paragraph         <- body1, muted

    [Primary Action]  [Secondary Action]
```

**Rules:**
- HTTP status code is the visual anchor, not an icon
- Left-aligned text (not centered -- avoids wobbly multi-line centering)
- Primary action uses `contained` amber/secondary color
- Secondary action uses `outlined`
- Subtle background gradient using primaryLight to reinforce brand
- Vertically centered in viewport
- Max content width ~480px

### Dashboard / Data Pages

(To be defined when first data page is built.)

### Form Pages

(To be defined when first form page is built.)

## Spacing & Layout

From `src/theme/constants.ts`:
- Header height: 64px
- Sidebar width: 280px (collapsed: 64px)
- Page padding: 16/24/32px (sm/md/lg breakpoints)
- Border radius scale: 0/2/4/8/12/16/9999

## Dark Mode

Dark mode is a first-class citizen, not an afterthought:
- Background: `#0b0f19` (near-black with blue undertone matching navy identity)
- Paper: `#111827` (dark blue-grey)
- Elevation surfaces: `level1` (#1e2536) and `level2` (#262d3f) for cards-on-cards
- Borders use `alpha('#fff', 0.1)` not hard-coded greys
- Status colors are brightened for readability on dark backgrounds