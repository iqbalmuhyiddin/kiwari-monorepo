# Kiwari POS - Android App

Android POS application for Kiwari F&B business.

## Tech Stack

- **Language:** Kotlin
- **UI Framework:** Jetpack Compose with Material 3
- **Architecture:** MVVM with Clean Architecture
- **Dependency Injection:** Hilt
- **Networking:** Retrofit + OkHttp
- **Local Storage:** DataStore Preferences
- **Navigation:** Navigation Compose
- **Min SDK:** 26 (Android 8.0)
- **Target SDK:** 34

## Project Structure

```
app/src/main/java/com/kiwari/pos/
├── di/              # Hilt dependency injection modules
├── data/
│   ├── api/         # Retrofit API interfaces
│   ├── model/       # API response models
│   └── repository/  # Data repositories
├── domain/
│   └── model/       # Domain models
├── ui/
│   ├── theme/       # Kiwari design tokens (colors, typography)
│   ├── login/       # Login screen
│   ├── menu/        # Menu list screen
│   ├── cart/        # Cart screen
│   ├── payment/     # Payment screen
│   ├── catering/    # Catering booking screen
│   └── components/  # Shared composables
└── util/            # Helpers (printing, etc.)
```

## Design Tokens

Kiwari brand colors are defined in `ui/theme/KiwariColors.kt`:

- **Primary Green:** `#0C7721` - Actions, success, primary buttons
- **Primary Yellow:** `#FFD500` - Headers, highlights, accents
- **Border Yellow:** `#FFEA60` - Active states, subtle borders
- **Accent Red:** `#D43B0A` - Errors, destructive actions
- **Dark Grey:** `#262626` - Text in light mode, background in dark mode
- **Cream Light:** `#FFFCF2` - Background in light mode

Font: **Inter** (with Roboto fallback)

## Setup

### Prerequisites

- Android Studio Ladybug (2024.2.1) or newer
- JDK 17
- Kotlin 2.1.0+

**Important - Gradle Wrapper Setup:**

After cloning this repository, the Gradle wrapper JAR (`gradle/wrapper/gradle-wrapper.jar`) is not included in version control. You must generate it before building:

**Option 1 (Recommended):** Open the project in Android Studio, which will automatically download the wrapper JAR during project sync.

**Option 2 (Manual):** If you have Gradle installed separately, run:
```bash
gradle wrapper --gradle-version 8.9
```

This is an environment limitation - we cannot generate binary JAR files in the codebase, so this step is required on first clone.

### Building

```bash
# From monorepo root
make android-build

# Or directly with Gradle
cd android
./gradlew assembleDebug
```

### Running Tests

```bash
make android-test

# Or directly
cd android
./gradlew test
```

## API Configuration

The app connects to different API endpoints based on build type:

- **Debug:** `http://10.0.2.2:8081/api/v1/` (for emulator; `10.0.2.2` is the emulator's alias for the host machine's `localhost`)
- **Release:** `https://pos-api.nasibakarkiwari.com/api/v1/` (production)

These are configured in `app/build.gradle.kts` as BuildConfig fields.

**Note:** If testing on a physical device, you'll need to update the debug URL to your machine's local network IP address (e.g., `http://192.168.1.100:8081/api/v1/`).

## Missing Assets

Before building, you need to:

1. **Add Inter font files** to `app/src/main/res/font/`:
   - `inter_regular.ttf`
   - `inter_medium.ttf`
   - `inter_semibold.ttf`
   - `inter_bold.ttf`

   Download from: https://fonts.google.com/specimen/Inter

2. **Generate launcher icons** for all densities (mdpi, hdpi, xhdpi, xxhdpi, xxxhdpi)
   - Use Android Studio's Image Asset tool
   - Brand colors: Primary Green (#0C7721) and Primary Yellow (#FFD500)

## Implementation Progress

- [x] Project scaffold with Gradle version catalogs
- [x] Hilt setup with NetworkModule
- [x] Compose theme with Kiwari brand tokens
- [x] Minimal MainActivity
- [ ] Login screen (Task 8.2)
- [ ] Menu list screen
- [ ] Cart and payment flow
- [ ] Catering booking
- [ ] Order history
- [ ] Kitchen display

## Notes

- Using **KSP** (not kapt) for Hilt annotation processing
- Using **BigDecimal** for all money calculations
- All API calls use Kotlin Coroutines
- WebSocket support via OkHttp for live order updates
