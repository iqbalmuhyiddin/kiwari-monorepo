package com.kiwari.pos.util

import java.math.BigDecimal
import java.text.NumberFormat
import java.util.Locale

private val rupiahFormatter: NumberFormat = NumberFormat.getCurrencyInstance(Locale("id", "ID")).apply {
    maximumFractionDigits = 0
}

fun formatPrice(price: BigDecimal): String {
    return rupiahFormatter.format(price)
}
