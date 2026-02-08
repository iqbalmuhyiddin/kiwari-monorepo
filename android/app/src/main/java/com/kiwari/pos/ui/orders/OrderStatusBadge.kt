package com.kiwari.pos.ui.orders

import androidx.compose.foundation.background
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.layout.padding

@Composable
fun OrderStatusBadge(status: String) {
    val (label, containerColor, labelColor) = when (status.uppercase()) {
        "NEW" -> Triple(
            "Baru",
            MaterialTheme.colorScheme.primary,
            MaterialTheme.colorScheme.onPrimary
        )
        "PREPARING" -> Triple(
            "Diproses",
            MaterialTheme.colorScheme.secondary,
            MaterialTheme.colorScheme.onSecondary
        )
        "READY" -> Triple(
            "Siap",
            MaterialTheme.colorScheme.surfaceVariant,
            MaterialTheme.colorScheme.onSurfaceVariant
        )
        "COMPLETED" -> Triple(
            "Selesai",
            MaterialTheme.colorScheme.primary,
            MaterialTheme.colorScheme.onPrimary
        )
        "CANCELLED" -> Triple(
            "Batal",
            MaterialTheme.colorScheme.error,
            MaterialTheme.colorScheme.onError
        )
        else -> Triple(
            status,
            MaterialTheme.colorScheme.surfaceVariant,
            MaterialTheme.colorScheme.onSurfaceVariant
        )
    }

    Text(
        text = label,
        style = MaterialTheme.typography.labelSmall,
        fontWeight = FontWeight.Medium,
        color = labelColor,
        modifier = Modifier
            .background(
                color = containerColor,
                shape = MaterialTheme.shapes.extraSmall
            )
            .padding(horizontal = 8.dp, vertical = 4.dp)
    )
}
