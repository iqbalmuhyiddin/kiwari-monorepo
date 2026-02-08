package com.kiwari.pos.util

import java.math.BigDecimal

/**
 * Returns this value if it's greater than or equal to [minimum], otherwise returns [minimum].
 * This is a BigDecimal-friendly version of the standard library's coerceAtLeast.
 */
fun BigDecimal.coerceAtLeast(minimum: BigDecimal): BigDecimal {
    return if (this < minimum) minimum else this
}
