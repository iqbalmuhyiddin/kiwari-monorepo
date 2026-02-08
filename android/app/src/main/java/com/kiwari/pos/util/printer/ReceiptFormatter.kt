package com.kiwari.pos.util.printer

import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.ui.payment.PaymentEntry
import com.kiwari.pos.ui.payment.PaymentMethod
import com.kiwari.pos.util.formatPrice
import com.kiwari.pos.util.parseBigDecimal
import java.math.BigDecimal
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter

/** Merge a list of byte arrays into one. */
internal fun List<ByteArray>.merge(): ByteArray {
    val totalSize = this.sumOf { it.size }
    val result = ByteArray(totalSize)
    var offset = 0
    for (arr in this) {
        arr.copyInto(result, offset)
        offset += arr.size
    }
    return result
}

/**
 * All data needed to print a receipt or kitchen ticket.
 */
data class ReceiptData(
    val outletName: String,
    val orderNumber: String,
    val orderType: String,
    val tableNumber: String = "",
    val cartItems: List<CartItem>,
    val subtotal: BigDecimal,
    val discountLabel: String? = null,
    val discountAmount: BigDecimal = BigDecimal.ZERO,
    val total: BigDecimal,
    val payments: List<PaymentEntry> = emptyList(),
    val changeAmount: BigDecimal = BigDecimal.ZERO,
    val orderNotes: String = "",
    val dateTime: LocalDateTime = LocalDateTime.now(),
    val isCatering: Boolean = false,
    val dpAmount: BigDecimal? = null
)

/**
 * Formats receipt and kitchen ticket data into ESC/POS byte arrays.
 */
object ReceiptFormatter {

    private val dateTimeFormatter = DateTimeFormatter.ofPattern("dd/MM/yyyy HH:mm")

    /**
     * Format a full customer receipt.
     */
    fun formatReceipt(data: ReceiptData, paperWidth: Int = EscPosCommands.WIDTH_58MM): ByteArray {
        val buf = mutableListOf<ByteArray>()
        val cmd = EscPosCommands

        buf += cmd.INIT

        // -- Header: Outlet name (centered, bold, double-height) --
        buf += cmd.ALIGN_CENTER
        buf += cmd.DOUBLE_HEIGHT_BOLD_ON
        buf += cmd.textLine(data.outletName)
        buf += cmd.DOUBLE_HEIGHT_OFF
        buf += cmd.BOLD_OFF
        buf += cmd.LF

        // -- Order number + date/time --
        buf += cmd.ALIGN_LEFT
        buf += cmd.twoColumnLine("No: ${data.orderNumber}", data.dateTime.format(dateTimeFormatter), paperWidth)

        // -- Order type + table --
        val orderTypeDisplay = formatOrderType(data.orderType)
        if (data.tableNumber.isNotBlank()) {
            buf += cmd.twoColumnLine(orderTypeDisplay, "Meja: ${data.tableNumber}", paperWidth)
        } else {
            buf += cmd.textLine(orderTypeDisplay)
        }

        buf += cmd.separator(paperWidth)

        // -- Items --
        for (item in data.cartItems) {
            // Item name with quantity and price
            val qty = "${item.quantity}x"
            val price = formatPrice(item.lineTotal)
            val name = item.product.name
            // Format: "2x Nasi Bakar             Rp30.000"
            val leftPart = "$qty $name"
            buf += cmd.twoColumnLine(leftPart, price, paperWidth)

            // Variants (indented)
            for (variant in item.selectedVariants) {
                val variantText = "  ${variant.variantGroupName}: ${variant.variantName}"
                if (variant.priceAdjustment.compareTo(BigDecimal.ZERO) != 0) {
                    buf += cmd.twoColumnLine(variantText, "+${formatPrice(variant.priceAdjustment)}", paperWidth)
                } else {
                    buf += cmd.textLine(variantText)
                }
            }

            // Modifiers (indented)
            for (modifier in item.selectedModifiers) {
                val modText = "  + ${modifier.modifierName}"
                if (modifier.price.compareTo(BigDecimal.ZERO) != 0) {
                    buf += cmd.twoColumnLine(modText, "+${formatPrice(modifier.price)}", paperWidth)
                } else {
                    buf += cmd.textLine(modText)
                }
            }

            // Item notes
            if (item.notes.isNotBlank()) {
                buf += cmd.textLine("  *${item.notes}")
            }
        }

        buf += cmd.separator(paperWidth)

        // -- Subtotal --
        buf += cmd.twoColumnLine("Subtotal", formatPrice(data.subtotal), paperWidth)

        // -- Discount --
        if (data.discountAmount.compareTo(BigDecimal.ZERO) != 0 && data.discountLabel != null) {
            buf += cmd.twoColumnLine("Diskon (${data.discountLabel})", "-${formatPrice(data.discountAmount)}", paperWidth)
        }

        // -- Total (bold) --
        buf += cmd.BOLD_ON
        buf += cmd.twoColumnLine("TOTAL", formatPrice(data.total), paperWidth)
        buf += cmd.BOLD_OFF

        buf += cmd.separator(paperWidth)

        // -- Payments --
        if (data.isCatering && data.dpAmount != null) {
            buf += cmd.twoColumnLine("DP", formatPrice(data.dpAmount), paperWidth)
        } else {
            for (payment in data.payments) {
                val amount = parseBigDecimal(payment.amount)
                if (amount.compareTo(BigDecimal.ZERO) <= 0) continue
                val methodLabel = formatPaymentMethod(payment.method)
                buf += cmd.twoColumnLine(methodLabel, formatPrice(amount), paperWidth)
            }
        }

        // -- Change --
        if (data.changeAmount.compareTo(BigDecimal.ZERO) > 0) {
            buf += cmd.twoColumnLine("Kembalian", formatPrice(data.changeAmount), paperWidth)
        }

        buf += cmd.separator(paperWidth)

        // -- Footer --
        buf += cmd.ALIGN_CENTER
        buf += cmd.textLine("Terima Kasih")
        buf += cmd.LF

        // Feed and cut
        buf += cmd.feedLines(3)
        buf += cmd.CUT

        return buf.merge()
    }

    /**
     * Format a kitchen ticket — items only, notes prominent.
     */
    fun formatKitchenTicket(data: ReceiptData, paperWidth: Int = EscPosCommands.WIDTH_58MM): ByteArray {
        val buf = mutableListOf<ByteArray>()
        val cmd = EscPosCommands

        buf += cmd.INIT

        // -- Order number (bold, double-height) --
        buf += cmd.ALIGN_CENTER
        buf += cmd.DOUBLE_HEIGHT_BOLD_ON
        buf += cmd.textLine("#${data.orderNumber}")
        buf += cmd.DOUBLE_HEIGHT_OFF
        buf += cmd.BOLD_OFF

        // -- Order type + table --
        buf += cmd.ALIGN_LEFT
        val orderTypeDisplay = formatOrderType(data.orderType)
        if (data.tableNumber.isNotBlank()) {
            buf += cmd.twoColumnLine(orderTypeDisplay, "Meja: ${data.tableNumber}", paperWidth)
        } else {
            buf += cmd.textLine(orderTypeDisplay)
        }
        buf += cmd.textLine(data.dateTime.format(dateTimeFormatter))

        buf += cmd.separator(paperWidth)

        // -- Items with qty prominent --
        for (item in data.cartItems) {
            buf += cmd.BOLD_ON
            buf += cmd.textLine("${item.quantity}x ${item.product.name}")
            buf += cmd.BOLD_OFF

            // Variants
            for (variant in item.selectedVariants) {
                buf += cmd.textLine("  ${variant.variantGroupName}: ${variant.variantName}")
            }

            // Modifiers
            for (modifier in item.selectedModifiers) {
                buf += cmd.textLine("  + ${modifier.modifierName}")
            }

            // Item notes — PROMINENT (bold)
            if (item.notes.isNotBlank()) {
                buf += cmd.BOLD_ON
                buf += cmd.textLine("  >> ${item.notes}")
                buf += cmd.BOLD_OFF
            }
        }

        // -- Order-level notes --
        if (data.orderNotes.isNotBlank()) {
            buf += cmd.separator(paperWidth)
            buf += cmd.BOLD_ON
            buf += cmd.textLine("CATATAN: ${data.orderNotes}")
            buf += cmd.BOLD_OFF
        }

        // Feed and cut
        buf += cmd.feedLines(3)
        buf += cmd.CUT

        return buf.merge()
    }

    private fun formatOrderType(type: String): String = when (type) {
        "DINE_IN" -> "Dine In"
        "TAKEAWAY" -> "Takeaway"
        "DELIVERY" -> "Delivery"
        "CATERING" -> "Catering"
        else -> type
    }

    private fun formatPaymentMethod(method: PaymentMethod): String = when (method) {
        PaymentMethod.CASH -> "Tunai"
        PaymentMethod.QRIS -> "QRIS"
        PaymentMethod.TRANSFER -> "Transfer"
    }
}
