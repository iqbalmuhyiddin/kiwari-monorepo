package com.kiwari.pos.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

private val LightColorScheme = lightColorScheme(
    primary = PrimaryGreen,
    onPrimary = White,
    primaryContainer = PrimaryGreen.copy(alpha = 0.1f),
    onPrimaryContainer = PrimaryGreen,

    secondary = PrimaryYellow,
    onSecondary = DarkGrey,
    secondaryContainer = PrimaryYellow.copy(alpha = 0.2f),
    onSecondaryContainer = DarkGrey,

    tertiary = BorderYellow,
    onTertiary = DarkGrey,
    tertiaryContainer = BorderYellow.copy(alpha = 0.2f),
    onTertiaryContainer = DarkGrey,

    error = AccentRed,
    onError = White,
    errorContainer = AccentRed.copy(alpha = 0.1f),
    onErrorContainer = AccentRed,

    background = CreamLight,
    onBackground = DarkGrey,

    surface = White,
    onSurface = DarkGrey,
    surfaceVariant = LightGrey,
    onSurfaceVariant = DarkGrey.copy(alpha = 0.7f),

    outline = MediumGrey,
    outlineVariant = MediumGrey.copy(alpha = 0.4f)
)

private val DarkColorScheme = darkColorScheme(
    primary = PrimaryGreen,
    onPrimary = White,
    primaryContainer = PrimaryGreen.copy(alpha = 0.3f),
    onPrimaryContainer = PrimaryGreen,

    secondary = PrimaryYellow,
    onSecondary = DarkGrey,
    secondaryContainer = PrimaryYellow.copy(alpha = 0.3f),
    onSecondaryContainer = PrimaryYellow,

    tertiary = BorderYellow,
    onTertiary = DarkGrey,
    tertiaryContainer = BorderYellow.copy(alpha = 0.3f),
    onTertiaryContainer = BorderYellow,

    error = AccentRed,
    onError = White,
    errorContainer = AccentRed.copy(alpha = 0.3f),
    onErrorContainer = AccentRed.copy(alpha = 0.8f),

    background = DarkBackground,
    onBackground = CreamLight,

    surface = DarkSurface,
    onSurface = CreamLight,
    surfaceVariant = SurfaceGrey,
    onSurfaceVariant = CreamLight.copy(alpha = 0.7f),

    outline = MediumGrey,
    outlineVariant = MediumGrey.copy(alpha = 0.6f)
)

@Composable
fun KiwariTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    // Dynamic color is available on Android 12+
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val colorScheme = when {
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            val context = LocalContext.current
            if (darkTheme) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        }
        darkTheme -> DarkColorScheme
        else -> LightColorScheme
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = KiwariTypography,
        content = content
    )
}
