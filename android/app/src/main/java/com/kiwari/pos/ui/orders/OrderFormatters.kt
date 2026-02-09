package com.kiwari.pos.ui.orders

import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

val orderTimestampFormatter: DateTimeFormatter = DateTimeFormatter.ofPattern("dd MMM yyyy, HH:mm")

fun formatOrderTimestamp(isoTimestamp: String): String {
    return try {
        val instant = Instant.parse(isoTimestamp)
        val localDateTime = instant.atZone(ZoneId.systemDefault()).toLocalDateTime()
        localDateTime.format(orderTimestampFormatter)
    } catch (_: Exception) {
        isoTimestamp
    }
}
