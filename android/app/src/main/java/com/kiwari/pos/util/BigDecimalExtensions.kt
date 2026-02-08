package com.kiwari.pos.util

import java.math.BigDecimal

/**
 * Returns this value if it's greater than or equal to [minimum], otherwise returns [minimum].
 * This is a BigDecimal-friendly version of the standard library's coerceAtLeast.
 */
fun BigDecimal.coerceAtLeast(minimum: BigDecimal): BigDecimal {
    return if (this < minimum) minimum else this
}

/**
 * Parses a string to BigDecimal, returning ZERO if the string is blank or invalid.
 */
fun parseBigDecimal(value: String): BigDecimal {
    return try {
        if (value.isBlank()) BigDecimal.ZERO else BigDecimal(value)
    } catch (e: NumberFormatException) {
        BigDecimal.ZERO
    }
}

/**
 * Filters input to allow only digits and decimal point, ensuring only one decimal point exists.
 */
fun filterDecimalInput(input: String): String {
    val filtered = input.filter { it.isDigit() || it == '.' }
    if (filtered.count { it == '.' } > 1) {
        return filtered.substringBefore('.') + "." + filtered.substringAfter('.').replace(".", "")
    }
    return filtered
}
