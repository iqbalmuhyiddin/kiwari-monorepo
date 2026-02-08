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
