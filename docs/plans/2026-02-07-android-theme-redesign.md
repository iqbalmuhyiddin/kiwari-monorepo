# Android Theme Redesign — Bold + Clean

**Date:** 2026-02-07
**Status:** Approved
**Scope:** Theme-only change (KiwariColors.kt, KiwariTheme.kt, KiwariTypography.kt + component adjustments)
**Preview:** `docs/old-references/design-system/bold-clean-preview.html`

## Direction

Bold + Clean — keep strong brand colors (green/yellow) but with better spacing, typography, and restraint. Colors pop because the rest is quiet. Light mode only.

---

## Section 1: Color Palette

| Token | Old Value | New Value | Usage |
|-------|-----------|-----------|-------|
| Background | `#FFFCF2` (cream) | `#FFFFFF` (white) | Screen background |
| Surface | `#F5F5F5` | `#F8F9FA` (cool grey-50) | Cards bg, input bg, avatars |
| Primary | `#0C7721` | `#0C7721` (keep) | CTAs, selected states, checkboxes |
| Accent | `#FFD500` | `#FFD500` (keep) | Category chips (selected), header underline |
| Text Primary | `#262626` | `#1A1A1A` | Headings, product names, prices |
| Text Secondary | alpha hack | `#6B7280` | Subtitles, hints, captions |
| Border | `#E0E0E0` | `#E5E7EB` (grey-200) | Card borders, dividers |
| Error | `#D43B0A` | `#DC2626` (red-600) | Error states, destructive |
| On Primary | `#FFFFFF` | `#FFFFFF` | Text on green buttons |
| On Accent | `#262626` | `#1A1A1A` | Text on yellow chips |

**Removed colors:**
- `CreamLight` (#FFFCF2) — replaced by white
- `BorderYellow` (#FFEA60) — no longer used as border accent
- `SurfaceGrey` (#3A3838) — dark theme only
- `DarkBackground` (#1A1A1A) — dark theme only
- `DarkSurface` (#2D2D2D) — dark theme only

**Key principle:** Green and yellow reserved for meaning only. Everything else greyscale.

---

## Section 2: Typography

**Font:** Roboto (system default, no custom font files).

**3 visual levels:** Bold (headings, prices, CTAs), SemiBold (product names), Regular (everything else).

| Role | Weight | Size | Letter Spacing | M3 Slot |
|------|--------|------|----------------|---------|
| Screen heading | Bold (700) | 20sp | -0.3sp | titleLarge |
| Section heading | Bold (700) | 18sp | 0 | titleMedium |
| Product name | SemiBold (600) | 15sp | 0 | bodyLarge |
| Price | Bold (700) | 15sp | 0 | bodyLarge (custom) |
| Body text | Regular (400) | 13sp | 0 | bodyMedium |
| Caption/hint | Regular (400) | 12sp | 0.02em | bodySmall |
| Label (chips, badges) | Medium (500) | 12sp | 0.02em | labelMedium |
| Small label | Medium (500) | 11sp | 0.02em | labelSmall |

**Size range:** 11sp–20sp (tighter than default M3).

---

## Section 3: Shapes, Spacing & Elevation

### Corner Radius

| Element | Old | New |
|---------|-----|-----|
| Cards (product, cart item) | 20dp | **12dp** |
| Buttons (CTA, checkout) | 16dp | **10dp** |
| Chips (category filters) | 20dp (pill) | **8dp** (rounded rect) |
| Bottom sheets | 32dp | **16dp** |
| Input fields | 12dp | **8dp** |
| FAB / icon buttons | 50% | **50%** (keep) |

**Scale:** 8 / 10 / 12 / 16. No 20+ dp curves.

### Spacing (8dp Grid)

| Value | Usage |
|-------|-------|
| 4dp | Icon-to-text gap, tight internal |
| 8dp | Chip gaps, card-to-card spacing |
| 12dp | Card internal padding (vertical) |
| 16dp | Screen edge padding, between sections |
| 24dp | Section breaks (category bar to product list) |
| 32dp | Major sections (header to content) |

**No more:** 10dp, 15dp, 20dp. Everything snaps to 4/8/12/16/24/32.

### Elevation

| Element | Elevation | Style |
|---------|-----------|-------|
| Cards | **0dp** | 1px border (#E5E7EB), no shadow |
| Bottom bar (cart) | **4dp** | Shadow — only surface that floats |
| Bottom sheet | **8dp** | Shadow — modal overlay |
| Everything else | **0dp** | Flat |

**Principle:** Borders > shadows. Shadows reserved for truly floating elements.

---

## Section 4: Component Behavior & States

### Buttons

| State | Primary (green) | Secondary (outlined) |
|-------|----------------|---------------------|
| Default | `#0C7721` bg, white text | `#E5E7EB` border, `#1A1A1A` text, transparent bg |
| Pressed | 10% black overlay (darken) | `#F8F9FA` bg fill |
| Disabled | `#0C7721` at 38% alpha, text at 60% alpha | border + text at 38% alpha |

### Category Chips

| State | Appearance |
|-------|-----------|
| Unselected | `#F8F9FA` bg, `#E5E7EB` border, `#6B7280` text |
| Selected | `#FFD500` bg, `#FFD500` border, `#1A1A1A` text |

### Product Cards

| State | Appearance |
|-------|-----------|
| Default | White bg, `#E5E7EB` border, 12dp radius, 0 elevation |
| Pressed | `#F8F9FA` bg, border stays, no color change |
| With cart items | Green badge with count on the + button |

### Letter Avatars

| Property | Old | New |
|----------|-----|-----|
| Size | 64dp | 56dp |
| Background | `#EEE` | `#F8F9FA` |
| Radius | 12dp | 8dp |
| Text color | `#262626` (dark) | `#6B7280` (grey) |

### Text Fields

| State | Appearance |
|-------|-----------|
| Default | `#F8F9FA` bg, `#E5E7EB` 1px border, 8dp radius |
| Focused | White bg, `#0C7721` 2px border |
| Error | `#FEF2F2` bg tint, `#DC2626` 2px border |

### Selection Controls (Radio / Checkbox)

| State | Appearance |
|-------|-----------|
| Unchecked | `#E5E7EB` 2px border, no fill |
| Checked (radio) | `#0C7721` fill, white inset ring |
| Checked (checkbox) | `#0C7721` fill, white checkmark |
| Selected row | `#0C7721` border, `rgba(12,119,33,0.04)` bg tint |

---

## Implementation Notes

### Files to modify:
1. **KiwariColors.kt** — Replace color definitions, remove dark-only colors
2. **KiwariTheme.kt** — Remove DarkColorScheme, update LightColorScheme mapping, add Shapes
3. **KiwariTypography.kt** — Tighten all sizes to 11-20sp range, update weights

### Files to adjust (component-level):
4. **LoginScreen.kt** — Input field styling, button radius
5. **MenuScreen.kt** — Chip radius, card radius/spacing/elevation, avatar size
6. **CustomizationScreen.kt** — Option row borders, radio/checkbox colors
7. **CartBottomBar.kt** — Elevation, button color (green instead of dark)
8. **Any hardcoded colors/dimensions** — Grep for `0xFF`, `.dp`, `RoundedCornerShape`

### What NOT to change:
- No new files needed (except possibly a `KiwariShapes.kt` if we want)
- No layout restructuring — this is purely token/styling
- No logic changes
- No dark theme (remove entirely)

### M3 ColorScheme mapping:

```kotlin
lightColorScheme(
    primary = Color(0xFF0C7721),        // Green — CTAs, selections
    onPrimary = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFF0C7721).copy(alpha = 0.04f),  // Subtle green tint
    secondary = Color(0xFFFFD500),       // Yellow — chips, accents
    onSecondary = Color(0xFF1A1A1A),
    background = Color(0xFFFFFFFF),      // Pure white
    onBackground = Color(0xFF1A1A1A),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF1A1A1A),
    surfaceVariant = Color(0xFFF8F9FA),  // Cool grey for inputs, avatars
    onSurfaceVariant = Color(0xFF6B7280), // Secondary text
    outline = Color(0xFFE5E7EB),         // Borders
    error = Color(0xFFDC2626),
    onError = Color(0xFFFFFFFF),
)
```
