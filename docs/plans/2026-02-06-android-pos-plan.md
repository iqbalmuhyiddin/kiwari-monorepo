# Android POS App Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the cashier-facing Android POS app with Jetpack Compose for taking orders, processing payments, and printing receipts.

**Architecture:** Kotlin Android app using Jetpack Compose for UI, Hilt for DI, Retrofit+OkHttp for API calls and WebSocket. Communicates with the Go REST API deployed at `pos-api.nasibakarkiwari.com`.

**Tech Stack:** Kotlin, Jetpack Compose, Hilt, Retrofit + OkHttp, DataStore, Min SDK 26, Target SDK 34

**Design Doc:** `docs/plans/2026-02-06-pos-system-design.md`

**Parent Plan:** `docs/plans/2026-02-06-backend-plan.md` (Milestones 1-7 complete, API ready)

---

## Milestone 8: Android POS App (Kotlin)

> Cashier-facing Android app with Jetpack Compose.

### Task 8.1: Scaffold Android Project

**Setup:**
- Android Studio project in `android/`
- Min SDK 26 (Android 8.0), target SDK 34
- Jetpack Compose for UI
- Hilt for dependency injection
- Retrofit + OkHttp for API calls
- OkHttp WebSocket for live updates
- DataStore for local preferences (auth tokens)

**Project structure:**
```
android/app/src/main/java/com/kiwari/pos/
├── di/              # Hilt modules
├── data/
│   ├── api/         # Retrofit interfaces
│   ├── model/       # API response models
│   └── repository/  # Data repositories
├── domain/
│   └── model/       # Domain models
├── ui/
│   ├── theme/       # Kiwari design tokens
│   ├── login/       # Login screen
│   ├── menu/        # Menu list screen
│   ├── cart/        # Cart screen
│   ├── payment/     # Payment screen
│   ├── catering/    # Catering booking screen
│   └── components/  # Shared composables
└── util/            # Helpers (printing, etc.)
```

Design tokens in Compose theme:
```kotlin
// KiwariColors.kt
val PrimaryGreen = Color(0xFF0C7721)
val PrimaryYellow = Color(0xFFFFD500)
val BorderYellow = Color(0xFFFFEA60)
val AccentRed = Color(0xFFD43B0A)
val DarkGrey = Color(0xFF262626)
val SurfaceGrey = Color(0xFF3A3838)
val CreamLight = Color(0xFFFFFCF2)
```

**Commit:** `feat: scaffold Android POS project with Compose and Hilt`

---

### Task 8.2: Login Screen

**Files:**
- Create: `ui/login/LoginScreen.kt`
- Create: `ui/login/LoginViewModel.kt`
- Create: `data/api/AuthApi.kt`
- Create: `data/repository/AuthRepository.kt`

Implement:
- Email + password login
- Quick PIN login
- Store JWT in encrypted DataStore
- Auto-refresh token on 401

**Commit:** `feat: add Android login screen with JWT auth`

---

### Task 8.3: Menu Screen (KasirPintar-style)

**Files:**
- Create: `ui/menu/MenuScreen.kt`
- Create: `ui/menu/MenuViewModel.kt`
- Create: `ui/menu/components/ProductListItem.kt`
- Create: `ui/menu/components/CategoryChips.kt`
- Create: `ui/menu/components/CartBottomBar.kt`
- Create: `ui/menu/components/QuickEditPopup.kt`
- Create: `data/api/MenuApi.kt`
- Create: `data/repository/MenuRepository.kt`

Implement:
- Full-width product list with letter avatar thumbnails
- Horizontal scrollable category chips
- Tap behavior:
  - Simple product → +1 qty instantly, badge appears
  - Product with required variants → customization bottom sheet
- Long-press → quick popup (qty +/-, add-on, discount, note)
- Qty badge on right side of list item
- Sticky bottom bar with item count + total + "LANJUT" button
- Search functionality

**Commit:** `feat: add menu screen with tap/long-press interactions`

---

### Task 8.4: Product Customization Bottom Sheet

**Files:**
- Create: `ui/menu/components/CustomizationSheet.kt`

Implement:
- Variant group selection (radio buttons per group)
- Modifier selection (checkboxes with min/max enforcement)
- Quantity selector
- Item note field
- "ADD TO CART" button with calculated price

**Commit:** `feat: add product customization bottom sheet`

---

### Task 8.5: Cart Screen

**Files:**
- Create: `ui/cart/CartScreen.kt`
- Create: `ui/cart/CartViewModel.kt`
- Create: `ui/cart/components/CartItem.kt`

Implement:
- Separate full page (not bottom sheet)
- Order type selector (Dine-in / Takeaway / Delivery / Catering)
- Table number input (for dine-in)
- Customer search/add
- Cart item list with:
  - Variant + modifier summary
  - Edit / delete buttons
  - Qty adjuster
- Order-level discount
- Subtotal / discount / total summary
- "BAYAR" button

**Commit:** `feat: add cart screen with order type and discount`

---

### Task 8.6: Payment Screen

**Files:**
- Create: `ui/payment/PaymentScreen.kt`
- Create: `ui/payment/PaymentViewModel.kt`

Implement:
- Multi-payment: add multiple payment methods
- Per payment: method selector (CASH/QRIS/TRANSFER)
- Cash: amount received → auto-calculate change
- QRIS/Transfer: reference number input
- Running total: paid vs remaining
- "SELESAI & CETAK" button → creates order + payments via API

**Commit:** `feat: add multi-payment screen`

---

### Task 8.7: Catering Booking Screen

**Files:**
- Create: `ui/catering/CateringScreen.kt`
- Create: `ui/catering/CateringViewModel.kt`

Implement:
- Customer selection (required)
- Date picker for catering date
- Delivery address
- DP amount display (50% of total)
- DP payment entry
- "BOOK & RECORD DP" button

**Commit:** `feat: add catering booking screen with down payment`

---

### Task 8.8: Thermal Printer Integration

**Files:**
- Create: `util/printer/ThermalPrinter.kt`
- Create: `util/printer/ReceiptFormatter.kt`

Implement:
- Bluetooth device scanning and pairing
- ESC/POS command generation for receipt
- Receipt format: outlet name, order number, items with variants/modifiers, totals, payment breakdown, date/time
- Kitchen ticket format: order number, items only, notes prominent
- Auto-print on order completion
- Settings screen for printer selection

**Commit:** `feat: add Bluetooth thermal printer with receipt formatting`

---

## Notes for Implementer

- **Android:** Requires Android Studio for build/test. CI can use GitHub Actions with Android emulator.
- **Decimal handling:** Use `BigDecimal` for all money fields. Never use `Double` or `Float`.
- **API base URL:** Configurable via BuildConfig — `pos-api.nasibakarkiwari.com` for production, `localhost:8081` for dev.
- **Auth flow:** JWT access token (15min) + refresh token (7 days). Store in EncryptedSharedPreferences or DataStore. OkHttp interceptor handles 401 → refresh → retry.
- **WebSocket:** OkHttp WebSocket client for live order updates. Connect to `ws/outlets/:oid/orders` with JWT as query param.
- **Brand design tokens:** Primary Green `#0c7721`, Primary Yellow `#ffd500`, Border Yellow `#ffea60`, Accent Red `#d43b0a`, Dark Grey `#262626`. Font: Inter (Roboto fallback).
