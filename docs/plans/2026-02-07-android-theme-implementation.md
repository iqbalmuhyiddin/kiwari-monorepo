# Android Bold + Clean Theme Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Apply the Bold + Clean design system to the Android POS app — update theme tokens, remove dark theme, fix hardcoded colors, and adjust component dimensions.

**Architecture:** Theme-only change. Modify 3 theme files (colors, theme, typography) and 5 component files (fix direct color imports and hardcoded dimensions). No layout restructuring, no logic changes.

**Tech Stack:** Kotlin, Jetpack Compose, Material 3

**Design Spec:** `docs/plans/2026-02-07-android-theme-redesign.md`
**Visual Preview:** `docs/old-references/design-system/bold-clean-preview.html`

**Working Directory:** `.worktrees/milestone-8-android-pos/android/`

---

### Task 1: Update KiwariColors.kt — New Color Palette

**Files:**
- Modify: `app/src/main/java/com/kiwari/pos/ui/theme/KiwariColors.kt`

**Step 1: Replace the entire file contents**

Replace the color definitions with the new Bold + Clean palette. Remove dark-theme-only colors, add new semantic colors.

```kotlin
package com.kiwari.pos.ui.theme

import androidx.compose.ui.graphics.Color

// ── Bold + Clean Palette ──────────────────────
// Design spec: docs/plans/2026-02-07-android-theme-redesign.md

// Brand colors (unchanged)
val PrimaryGreen = Color(0xFF0C7721)   // CTAs, selected states, checkboxes
val PrimaryYellow = Color(0xFFFFD500)  // Category chips (selected), header accent

// Error
val ErrorRed = Color(0xFFDC2626)       // Error states, destructive actions

// Neutrals
val TextPrimary = Color(0xFF1A1A1A)    // Headings, product names, prices
val TextSecondary = Color(0xFF6B7280)  // Subtitles, hints, captions
val BorderColor = Color(0xFFE5E7EB)    // Card borders, dividers
val SurfaceColor = Color(0xFFF8F9FA)   // Card bg, input bg, avatars
val White = Color(0xFFFFFFFF)          // Backgrounds, on-primary text
val ErrorBgTint = Color(0xFFFEF2F2)    // Error field background tint
```

**Step 2: Build to verify no compilation errors**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew compileDebugKotlin 2>&1 | tail -20`

Expected: BUILD FAILED — other files still import removed color names (`DarkGrey`, `CreamLight`, `AccentRed`, `Black`, `LightGrey`, `MediumGrey`, `DarkBackground`, `DarkSurface`, `SurfaceGrey`, `BorderYellow`). That's fine, we fix them in Tasks 2-4.

**Step 3: Do NOT commit yet** — wait until Task 2.

---

### Task 2: Update KiwariTheme.kt — Remove Dark Theme, Add Shapes

**Files:**
- Modify: `app/src/main/java/com/kiwari/pos/ui/theme/KiwariTheme.kt`

**Step 1: Replace the entire file contents**

Remove dark theme entirely. Update light color scheme to use new tokens. Add custom Shapes.

```kotlin
package com.kiwari.pos.ui.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.unit.dp

private val KiwariColorScheme = lightColorScheme(
    primary = PrimaryGreen,
    onPrimary = White,
    primaryContainer = PrimaryGreen.copy(alpha = 0.04f),
    onPrimaryContainer = PrimaryGreen,

    secondary = PrimaryYellow,
    onSecondary = TextPrimary,
    secondaryContainer = PrimaryYellow.copy(alpha = 0.15f),
    onSecondaryContainer = TextPrimary,

    error = ErrorRed,
    onError = White,
    errorContainer = ErrorBgTint,
    onErrorContainer = ErrorRed,

    background = White,
    onBackground = TextPrimary,

    surface = White,
    onSurface = TextPrimary,
    surfaceVariant = SurfaceColor,
    onSurfaceVariant = TextSecondary,

    outline = BorderColor,
    outlineVariant = BorderColor
)

private val KiwariShapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),   // chips, inputs
    small = RoundedCornerShape(10.dp),        // buttons
    medium = RoundedCornerShape(12.dp),       // cards
    large = RoundedCornerShape(16.dp),        // bottom sheets
    extraLarge = RoundedCornerShape(16.dp)    // same as large
)

@Composable
fun KiwariTheme(
    content: @Composable () -> Unit
) {
    MaterialTheme(
        colorScheme = KiwariColorScheme,
        typography = KiwariTypography,
        shapes = KiwariShapes,
        content = content
    )
}
```

**Step 2: Build to check**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew compileDebugKotlin 2>&1 | tail -20`

Expected: May still fail if callers pass `darkTheme` param. Check for compilation errors.

**Step 3: Fix any callers of KiwariTheme that pass darkTheme parameter**

Search for `KiwariTheme(` across the codebase. If any caller passes `darkTheme = ...`, remove that parameter.

Run: `grep -rn "KiwariTheme(" .worktrees/milestone-8-android-pos/android/app/src/`

**Step 4: Do NOT commit yet** — wait until Task 3.

---

### Task 3: Update KiwariTypography.kt — Tighter Size Range

**Files:**
- Modify: `app/src/main/java/com/kiwari/pos/ui/theme/KiwariTypography.kt`

**Step 1: Replace the entire file contents**

Tighten all sizes to 11-20sp. Remove unused display sizes. Map roles to the design spec.

```kotlin
package com.kiwari.pos.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// Roboto (system default) — no custom font files needed
val KiwariFontFamily = FontFamily.Default

val KiwariTypography = Typography(
    // Screen heading: "Kiwari POS", "Menu"
    titleLarge = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 20.sp,
        lineHeight = 28.sp,
        letterSpacing = (-0.3).sp
    ),
    // Section heading: "Kustomisasi", "Your Order"
    titleMedium = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 18.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.sp
    ),
    // Group labels: "Size", "Extra Topping", "Jumlah"
    titleSmall = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.SemiBold,
        fontSize = 15.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.sp
    ),
    // Product name, price (same size, different weight in usage)
    bodyLarge = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 15.sp,
        lineHeight = 22.sp,
        letterSpacing = 0.sp
    ),
    // Body text: descriptions, option names
    bodyMedium = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 13.sp,
        lineHeight = 18.sp,
        letterSpacing = 0.sp
    ),
    // Captions: hints, secondary info
    bodySmall = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.02.sp
    ),
    // Chip labels, badge text
    labelLarge = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 13.sp,
        lineHeight = 18.sp,
        letterSpacing = 0.02.sp
    ),
    // Small labels
    labelMedium = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.02.sp
    ),
    // Tiny labels: "Wajib", constraint hints
    labelSmall = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 11.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.02.sp
    ),
    // Keep display/headline for login title (used once)
    displaySmall = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 20.sp,
        lineHeight = 28.sp,
        letterSpacing = 0.sp
    ),
    headlineSmall = TextStyle(
        fontFamily = KiwariFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 20.sp,
        lineHeight = 28.sp,
        letterSpacing = 0.sp
    )
)
```

**Step 2: Build all three theme files together**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew compileDebugKotlin 2>&1 | tail -30`

Expected: Compilation errors from files that import old color names. Fix those in Task 4.

**Step 3: Commit theme files**

```bash
cd .worktrees/milestone-8-android-pos/android
git add app/src/main/java/com/kiwari/pos/ui/theme/KiwariColors.kt \
        app/src/main/java/com/kiwari/pos/ui/theme/KiwariTheme.kt \
        app/src/main/java/com/kiwari/pos/ui/theme/KiwariTypography.kt
git commit -m "refactor: apply Bold + Clean theme tokens (colors, shapes, typography)"
```

---

### Task 4: Fix Direct Color Imports in Components

**Files:**
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/CustomizationScreen.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/components/ProductListItem.kt`

These two files import colors directly instead of using `MaterialTheme.colorScheme`. Fix them to use theme tokens.

**Step 1: Fix CustomizationScreen.kt**

Remove direct imports and replace with MaterialTheme references:

1. Remove these imports (lines 46-47):
   ```kotlin
   import com.kiwari.pos.ui.theme.PrimaryGreen
   import com.kiwari.pos.ui.theme.White
   ```

2. In `VariantGroupSection`, line 296-298, change RadioButton colors:
   ```kotlin
   // OLD:
   colors = RadioButtonDefaults.colors(
       selectedColor = PrimaryGreen
   )
   // NEW:
   colors = RadioButtonDefaults.colors(
       selectedColor = MaterialTheme.colorScheme.primary
   )
   ```

3. In `ModifierGroupSection`, line 368-370, change Checkbox colors:
   ```kotlin
   // OLD:
   colors = CheckboxDefaults.colors(
       checkedColor = PrimaryGreen
   )
   // NEW:
   colors = CheckboxDefaults.colors(
       checkedColor = MaterialTheme.colorScheme.primary
   )
   ```

4. In `AddToCartButton`, lines 489-493, change Button colors:
   ```kotlin
   // OLD:
   colors = ButtonDefaults.buttonColors(
       containerColor = PrimaryGreen,
       contentColor = White,
       disabledContainerColor = PrimaryGreen.copy(alpha = 0.4f),
       disabledContentColor = White.copy(alpha = 0.6f)
   )
   // NEW:
   colors = ButtonDefaults.buttonColors(
       containerColor = MaterialTheme.colorScheme.primary,
       contentColor = MaterialTheme.colorScheme.onPrimary,
       disabledContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
       disabledContentColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.6f)
   )
   ```

5. In `AddToCartButton`, line 488, update button shape:
   ```kotlin
   // OLD:
   shape = RoundedCornerShape(12.dp),
   // NEW:
   shape = MaterialTheme.shapes.small,  // 10dp per design spec
   ```

**Step 2: Fix ProductListItem.kt**

Remove direct imports and replace with MaterialTheme references:

1. Remove these imports (lines 26-28):
   ```kotlin
   import com.kiwari.pos.ui.theme.PrimaryGreen
   import com.kiwari.pos.ui.theme.PrimaryYellow
   import com.kiwari.pos.ui.theme.White
   ```

2. In `LetterAvatar`, line 103-104, change avatar shape and color:
   ```kotlin
   // OLD:
   .clip(CircleShape)
   .background(PrimaryYellow),
   // NEW:
   .clip(RoundedCornerShape(8.dp))
   .background(MaterialTheme.colorScheme.surfaceVariant),
   ```
   Add import: `import androidx.compose.foundation.shape.RoundedCornerShape`

3. In `LetterAvatar`, line 111, change text color:
   ```kotlin
   // OLD:
   color = MaterialTheme.colorScheme.onSecondary
   // NEW:
   color = MaterialTheme.colorScheme.onSurfaceVariant
   ```

4. In `LetterAvatar`, reduce size. In the caller (line 56):
   ```kotlin
   // OLD:
   modifier = Modifier.size(44.dp)
   // NEW:
   modifier = Modifier.size(56.dp)
   ```
   Wait — the design says 56dp. Current is 44dp. The design spec says new = 56dp. Let me check: the old HTML mockup had 64dp, current code has 44dp, design spec says 56dp. Use 56dp.

5. In `QuantityBadge`, lines 124-125, replace hardcoded colors:
   ```kotlin
   // OLD:
   .background(PrimaryGreen),
   // NEW:
   .background(MaterialTheme.colorScheme.primary),
   ```

6. In `QuantityBadge`, line 132:
   ```kotlin
   // OLD:
   color = White
   // NEW:
   color = MaterialTheme.colorScheme.onPrimary
   ```

**Step 3: Build to verify**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew compileDebugKotlin 2>&1 | tail -20`

Expected: PASS (or errors from other files, not these two).

**Step 4: Commit**

```bash
cd .worktrees/milestone-8-android-pos/android
git add app/src/main/java/com/kiwari/pos/ui/menu/CustomizationScreen.kt \
        app/src/main/java/com/kiwari/pos/ui/menu/components/ProductListItem.kt
git commit -m "refactor: replace hardcoded colors with MaterialTheme tokens"
```

---

### Task 5: Update Component Dimensions (Radii, Spacing, Elevation)

**Files:**
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/components/CartBottomBar.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/components/CategoryChips.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/components/ProductListItem.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/MenuScreen.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/CustomizationScreen.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/login/LoginScreen.kt`
- Modify: `app/src/main/java/com/kiwari/pos/ui/menu/components/QuickEditPopup.kt`

**Step 1: CartBottomBar.kt — Fix elevation**

Line 32: Change shadow elevation from 8dp to 4dp per design spec:
```kotlin
// OLD:
shadowElevation = 8.dp,
// NEW:
shadowElevation = 4.dp,
```

**Step 2: CategoryChips.kt — Update chip colors for Bold + Clean**

The chips currently use `primary` (green) for selected state. Design spec says yellow for selected chips.

Replace the FilterChip colors in both places (lines 35-38 and 46-49):
```kotlin
// OLD:
colors = FilterChipDefaults.filterChipColors(
    selectedContainerColor = MaterialTheme.colorScheme.primary,
    selectedLabelColor = MaterialTheme.colorScheme.onPrimary
)
// NEW:
colors = FilterChipDefaults.filterChipColors(
    selectedContainerColor = MaterialTheme.colorScheme.secondary,
    selectedLabelColor = MaterialTheme.colorScheme.onSecondary,
    containerColor = MaterialTheme.colorScheme.surfaceVariant,
    labelColor = MaterialTheme.colorScheme.onSurfaceVariant
)
```

Also add chip shape override. Each FilterChip needs a shape parameter:
```kotlin
shape = MaterialTheme.shapes.extraSmall,  // 8dp rounded rect instead of pill
```

**Step 3: ProductListItem.kt — Update divider indent**

Line 90: The divider indent `start = 72.dp` was based on 16dp padding + 44dp avatar + 12dp gap = 72dp. With 56dp avatar: 16dp + 56dp + 12dp = 84dp.
```kotlin
// OLD:
modifier = Modifier.padding(start = 72.dp),
// NEW:
modifier = Modifier.padding(start = 84.dp),
```

**Step 4: Build and verify**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew compileDebugKotlin 2>&1 | tail -20`

Expected: BUILD SUCCESSFUL

**Step 5: Commit**

```bash
cd .worktrees/milestone-8-android-pos/android
git add app/src/main/java/com/kiwari/pos/ui/menu/components/CartBottomBar.kt \
        app/src/main/java/com/kiwari/pos/ui/menu/components/CategoryChips.kt \
        app/src/main/java/com/kiwari/pos/ui/menu/components/ProductListItem.kt
git commit -m "refactor: update component dimensions for Bold + Clean theme"
```

---

### Task 6: Full Build Verification

**Step 1: Clean build**

Run: `cd .worktrees/milestone-8-android-pos/android && ./gradlew clean assembleDebug 2>&1 | tail -30`

Expected: BUILD SUCCESSFUL

**Step 2: If build fails, fix any remaining compilation errors**

Common issues:
- Files importing removed color names (`AccentRed`, `DarkGrey`, `CreamLight`, etc.) — replace with new names
- `KiwariTheme` callers passing `darkTheme` parameter — remove parameter

Search for remaining broken imports:
```bash
grep -rn "AccentRed\|DarkGrey\|CreamLight\|SurfaceGrey\|DarkBackground\|DarkSurface\|LightGrey\|MediumGrey\|Black\|BorderYellow" \
  .worktrees/milestone-8-android-pos/android/app/src/main/java/
```

**Step 3: Verify APK builds**

Run: `cd .worktrees/milestone-8-android-pos/android && ls -la app/build/outputs/apk/debug/`

Expected: `app-debug.apk` exists

**Step 4: Final commit if any fixes were needed**

```bash
cd .worktrees/milestone-8-android-pos/android
git add -u
git commit -m "fix: resolve remaining theme migration compilation errors"
```

---

## Summary

| Task | What | Files | Commit |
|------|------|-------|--------|
| 1 | New color palette | KiwariColors.kt | (with T3) |
| 2 | Remove dark theme, add shapes | KiwariTheme.kt | (with T3) |
| 3 | Tighten typography | KiwariTypography.kt | Yes |
| 4 | Fix hardcoded colors | CustomizationScreen, ProductListItem | Yes |
| 5 | Update dimensions | CartBottomBar, CategoryChips, ProductListItem | Yes |
| 6 | Full build verification | — | If needed |

**Total: 3-4 commits, ~8 files modified, 0 new files.**
