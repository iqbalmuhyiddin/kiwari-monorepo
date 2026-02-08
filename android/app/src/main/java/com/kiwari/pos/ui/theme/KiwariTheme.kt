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
