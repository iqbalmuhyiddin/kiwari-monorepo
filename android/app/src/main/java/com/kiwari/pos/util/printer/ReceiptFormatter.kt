package com.kiwari.pos.util.printer

import com.kiwari.pos.data.model.CartItem
import com.kiwari.pos.data.model.OrderDetailResponse
import com.kiwari.pos.data.model.OrderItemModifierResponse
import com.kiwari.pos.data.model.OrderItemResponse
import com.kiwari.pos.ui.payment.PaymentEntry
import com.kiwari.pos.ui.payment.PaymentMethod
import com.kiwari.pos.util.formatPrice
import com.kiwari.pos.util.parseBigDecimal
import java.math.BigDecimal
import java.time.Instant
import java.time.LocalDateTime
import java.time.ZoneId
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

    private fun formatPaymentMethodStr(method: String): String = when (method.uppercase()) {
        "CASH" -> "Tunai"
        "QRIS" -> "QRIS"
        "TRANSFER" -> "Transfer"
        else -> method
    }

    private fun parseOrderDateTime(isoTimestamp: String): String {
        return try {
            val instant = Instant.parse(isoTimestamp)
            val localDateTime = instant.atZone(ZoneId.systemDefault()).toLocalDateTime()
            localDateTime.format(dateTimeFormatter)
        } catch (_: Exception) {
            isoTimestamp
        }
    }

    // ── OrderDetailResponse-based formatters ──────────────────────

    /**
     * Format a bill for an unpaid order (for thermal printer).
     * Same layout as receipt but: header says "BILL", no payment section,
     * footer says "** BELUM DIBAYAR **" centered bold.
     */
    fun formatBill(
        order: OrderDetailResponse,
        outletName: String,
        paperWidth: Int = EscPosCommands.WIDTH_58MM
    ): ByteArray {
        val buf = mutableListOf<ByteArray>()
        val cmd = EscPosCommands

        buf += cmd.INIT

        // -- Header: Outlet name (centered, bold, double-height) --
        buf += cmd.ALIGN_CENTER
        buf += cmd.DOUBLE_HEIGHT_BOLD_ON
        buf += cmd.textLine(outletName)
        buf += cmd.DOUBLE_HEIGHT_OFF
        buf += cmd.BOLD_OFF
        buf += cmd.LF

        // -- Order number + date/time --
        buf += cmd.ALIGN_LEFT
        buf += cmd.twoColumnLine("No: ${order.orderNumber}", parseOrderDateTime(order.createdAt), paperWidth)

        // -- Order type + table --
        val orderTypeDisplay = formatOrderType(order.orderType)
        val table = order.tableNumber
        if (!table.isNullOrBlank()) {
            buf += cmd.twoColumnLine(orderTypeDisplay, "Meja: $table", paperWidth)
        } else {
            buf += cmd.textLine(orderTypeDisplay)
        }

        buf += cmd.separator(paperWidth)

        // -- Items --
        for (item in order.items) {
            appendItemEscPos(buf, item, paperWidth)
        }

        buf += cmd.separator(paperWidth)

        // -- Subtotal --
        buf += cmd.twoColumnLine("Subtotal", formatPrice(BigDecimal(order.subtotal)), paperWidth)

        // -- Discount --
        val discountAmt = BigDecimal(order.discountAmount)
        if (discountAmt.compareTo(BigDecimal.ZERO) > 0) {
            val discountLabel = when {
                order.discountType?.uppercase() == "PERCENTAGE" && order.discountValue != null ->
                    "Diskon (${order.discountValue}%)"
                else -> "Diskon"
            }
            buf += cmd.twoColumnLine(discountLabel, "-${formatPrice(discountAmt)}", paperWidth)
        }

        // -- Tax --
        val taxAmt = BigDecimal(order.taxAmount)
        if (taxAmt.compareTo(BigDecimal.ZERO) > 0) {
            buf += cmd.twoColumnLine("Pajak", formatPrice(taxAmt), paperWidth)
        }

        // -- Total (bold) --
        buf += cmd.BOLD_ON
        buf += cmd.twoColumnLine("TOTAL", formatPrice(BigDecimal(order.totalAmount)), paperWidth)
        buf += cmd.BOLD_OFF

        buf += cmd.separator(paperWidth)

        // -- Footer: BELUM DIBAYAR --
        buf += cmd.ALIGN_CENTER
        buf += cmd.BOLD_ON
        buf += cmd.textLine("** BELUM DIBAYAR **")
        buf += cmd.BOLD_OFF
        buf += cmd.LF

        // Feed and cut
        buf += cmd.feedLines(3)
        buf += cmd.CUT

        return buf.merge()
    }

    /**
     * Format a receipt for a paid order (for thermal printer).
     * Overload that works with API response data.
     */
    fun formatReceipt(
        order: OrderDetailResponse,
        outletName: String,
        paperWidth: Int = EscPosCommands.WIDTH_58MM
    ): ByteArray {
        val buf = mutableListOf<ByteArray>()
        val cmd = EscPosCommands

        buf += cmd.INIT

        // -- Header: Outlet name (centered, bold, double-height) --
        buf += cmd.ALIGN_CENTER
        buf += cmd.DOUBLE_HEIGHT_BOLD_ON
        buf += cmd.textLine(outletName)
        buf += cmd.DOUBLE_HEIGHT_OFF
        buf += cmd.BOLD_OFF
        buf += cmd.LF

        // -- Order number + date/time --
        buf += cmd.ALIGN_LEFT
        buf += cmd.twoColumnLine("No: ${order.orderNumber}", parseOrderDateTime(order.createdAt), paperWidth)

        // -- Order type + table --
        val orderTypeDisplay = formatOrderType(order.orderType)
        val table = order.tableNumber
        if (!table.isNullOrBlank()) {
            buf += cmd.twoColumnLine(orderTypeDisplay, "Meja: $table", paperWidth)
        } else {
            buf += cmd.textLine(orderTypeDisplay)
        }

        buf += cmd.separator(paperWidth)

        // -- Items --
        for (item in order.items) {
            appendItemEscPos(buf, item, paperWidth)
        }

        buf += cmd.separator(paperWidth)

        // -- Subtotal --
        buf += cmd.twoColumnLine("Subtotal", formatPrice(BigDecimal(order.subtotal)), paperWidth)

        // -- Discount --
        val discountAmt = BigDecimal(order.discountAmount)
        if (discountAmt.compareTo(BigDecimal.ZERO) > 0) {
            val discountLabel = when {
                order.discountType?.uppercase() == "PERCENTAGE" && order.discountValue != null ->
                    "Diskon (${order.discountValue}%)"
                else -> "Diskon"
            }
            buf += cmd.twoColumnLine(discountLabel, "-${formatPrice(discountAmt)}", paperWidth)
        }

        // -- Tax --
        val taxAmt = BigDecimal(order.taxAmount)
        if (taxAmt.compareTo(BigDecimal.ZERO) > 0) {
            buf += cmd.twoColumnLine("Pajak", formatPrice(taxAmt), paperWidth)
        }

        // -- Total (bold) --
        buf += cmd.BOLD_ON
        buf += cmd.twoColumnLine("TOTAL", formatPrice(BigDecimal(order.totalAmount)), paperWidth)
        buf += cmd.BOLD_OFF

        buf += cmd.separator(paperWidth)

        // -- Payments --
        for (payment in order.payments) {
            val amount = BigDecimal(payment.amount)
            if (amount.compareTo(BigDecimal.ZERO) <= 0) continue
            val methodLabel = formatPaymentMethodStr(payment.paymentMethod)
            buf += cmd.twoColumnLine(methodLabel, formatPrice(amount), paperWidth)
        }

        // -- Change (from cash payments) --
        val totalChange = order.payments
            .filter { it.changeAmount != null }
            .fold(BigDecimal.ZERO) { acc, p -> acc.add(BigDecimal(p.changeAmount!!)) }
        if (totalChange.compareTo(BigDecimal.ZERO) > 0) {
            buf += cmd.twoColumnLine("Kembalian", formatPrice(totalChange), paperWidth)
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
     * Format a kitchen ticket from API order data (for thermal printer).
     */
    fun formatKitchenTicket(
        order: OrderDetailResponse,
        paperWidth: Int = EscPosCommands.WIDTH_58MM
    ): ByteArray {
        val buf = mutableListOf<ByteArray>()
        val cmd = EscPosCommands

        buf += cmd.INIT

        // -- Order number (bold, double-height) --
        buf += cmd.ALIGN_CENTER
        buf += cmd.DOUBLE_HEIGHT_BOLD_ON
        buf += cmd.textLine("#${order.orderNumber}")
        buf += cmd.DOUBLE_HEIGHT_OFF
        buf += cmd.BOLD_OFF

        // -- Order type + table --
        buf += cmd.ALIGN_LEFT
        val orderTypeDisplay = formatOrderType(order.orderType)
        val table = order.tableNumber
        if (!table.isNullOrBlank()) {
            buf += cmd.twoColumnLine(orderTypeDisplay, "Meja: $table", paperWidth)
        } else {
            buf += cmd.textLine(orderTypeDisplay)
        }
        buf += cmd.textLine(parseOrderDateTime(order.createdAt))

        buf += cmd.separator(paperWidth)

        // -- Items with qty prominent --
        for (item in order.items) {
            buf += cmd.BOLD_ON
            buf += cmd.textLine("${item.quantity}x Item")
            buf += cmd.BOLD_OFF

            // Modifiers
            for (modifier in item.modifiers) {
                val modText = if (modifier.quantity > 1) {
                    "  + Modifier x${modifier.quantity}"
                } else {
                    "  + Modifier"
                }
                buf += cmd.textLine(modText)
            }

            // Item notes — PROMINENT (bold)
            if (!item.notes.isNullOrBlank()) {
                buf += cmd.BOLD_ON
                buf += cmd.textLine("  >> ${item.notes}")
                buf += cmd.BOLD_OFF
            }
        }

        // -- Order-level notes --
        if (!order.notes.isNullOrBlank()) {
            buf += cmd.separator(paperWidth)
            buf += cmd.BOLD_ON
            buf += cmd.textLine("CATATAN: ${order.notes}")
            buf += cmd.BOLD_OFF
        }

        // Feed and cut
        buf += cmd.feedLines(3)
        buf += cmd.CUT

        return buf.merge()
    }

    /** Helper: append a single order item to ESC/POS buffer. */
    private fun appendItemEscPos(buf: MutableList<ByteArray>, item: OrderItemResponse, paperWidth: Int) {
        val cmd = EscPosCommands
        val qty = "${item.quantity}x"
        val price = formatPrice(BigDecimal(item.subtotal))
        val leftPart = "$qty Item"
        buf += cmd.twoColumnLine(leftPart, price, paperWidth)

        // Unit price (if qty > 1, show per-unit)
        if (item.quantity > 1) {
            buf += cmd.textLine("  @ ${formatPrice(BigDecimal(item.unitPrice))}")
        }

        // Modifiers (indented)
        for (modifier in item.modifiers) {
            val modPrice = BigDecimal(modifier.unitPrice).multiply(BigDecimal(modifier.quantity))
            val modText = if (modifier.quantity > 1) {
                "  + Modifier x${modifier.quantity}"
            } else {
                "  + Modifier"
            }
            if (modPrice.compareTo(BigDecimal.ZERO) != 0) {
                buf += cmd.twoColumnLine(modText, "+${formatPrice(modPrice)}", paperWidth)
            } else {
                buf += cmd.textLine(modText)
            }
        }

        // Item notes
        if (!item.notes.isNullOrBlank()) {
            buf += cmd.textLine("  *${item.notes}")
        }
    }

    // ── Plain-text formatters (for image generation / sharing) ───

    /**
     * Format a bill as plain text (for image generation / sharing).
     * Same content as formatBill but returns String instead of ESC/POS bytes.
     */
    fun formatBillText(
        order: OrderDetailResponse,
        outletName: String,
        charWidth: Int = EscPosCommands.WIDTH_58MM
    ): String {
        val sb = StringBuilder()

        // -- Header --
        sb.appendLine(centerText(outletName, charWidth))
        sb.appendLine()

        // -- Order number + date/time --
        sb.appendLine(twoColumnText("No: ${order.orderNumber}", parseOrderDateTime(order.createdAt), charWidth))

        // -- Order type + table --
        val orderTypeDisplay = formatOrderType(order.orderType)
        val table = order.tableNumber
        if (!table.isNullOrBlank()) {
            sb.appendLine(twoColumnText(orderTypeDisplay, "Meja: $table", charWidth))
        } else {
            sb.appendLine(orderTypeDisplay)
        }

        sb.appendLine("-".repeat(charWidth))

        // -- Items --
        for (item in order.items) {
            appendItemText(sb, item, charWidth)
        }

        sb.appendLine("-".repeat(charWidth))

        // -- Subtotal --
        sb.appendLine(twoColumnText("Subtotal", formatPrice(BigDecimal(order.subtotal)), charWidth))

        // -- Discount --
        val discountAmt = BigDecimal(order.discountAmount)
        if (discountAmt.compareTo(BigDecimal.ZERO) > 0) {
            val discountLabel = when {
                order.discountType?.uppercase() == "PERCENTAGE" && order.discountValue != null ->
                    "Diskon (${order.discountValue}%)"
                else -> "Diskon"
            }
            sb.appendLine(twoColumnText(discountLabel, "-${formatPrice(discountAmt)}", charWidth))
        }

        // -- Tax --
        val taxAmt = BigDecimal(order.taxAmount)
        if (taxAmt.compareTo(BigDecimal.ZERO) > 0) {
            sb.appendLine(twoColumnText("Pajak", formatPrice(taxAmt), charWidth))
        }

        // -- Total --
        sb.appendLine(twoColumnText("TOTAL", formatPrice(BigDecimal(order.totalAmount)), charWidth))

        sb.appendLine("-".repeat(charWidth))

        // -- Footer: BELUM DIBAYAR --
        sb.appendLine(centerText("** BELUM DIBAYAR **", charWidth))

        return sb.toString()
    }

    /**
     * Format a receipt as plain text (for image generation / sharing).
     */
    fun formatReceiptText(
        order: OrderDetailResponse,
        outletName: String,
        charWidth: Int = EscPosCommands.WIDTH_58MM
    ): String {
        val sb = StringBuilder()

        // -- Header --
        sb.appendLine(centerText(outletName, charWidth))
        sb.appendLine()

        // -- Order number + date/time --
        sb.appendLine(twoColumnText("No: ${order.orderNumber}", parseOrderDateTime(order.createdAt), charWidth))

        // -- Order type + table --
        val orderTypeDisplay = formatOrderType(order.orderType)
        val table = order.tableNumber
        if (!table.isNullOrBlank()) {
            sb.appendLine(twoColumnText(orderTypeDisplay, "Meja: $table", charWidth))
        } else {
            sb.appendLine(orderTypeDisplay)
        }

        sb.appendLine("-".repeat(charWidth))

        // -- Items --
        for (item in order.items) {
            appendItemText(sb, item, charWidth)
        }

        sb.appendLine("-".repeat(charWidth))

        // -- Subtotal --
        sb.appendLine(twoColumnText("Subtotal", formatPrice(BigDecimal(order.subtotal)), charWidth))

        // -- Discount --
        val discountAmt = BigDecimal(order.discountAmount)
        if (discountAmt.compareTo(BigDecimal.ZERO) > 0) {
            val discountLabel = when {
                order.discountType?.uppercase() == "PERCENTAGE" && order.discountValue != null ->
                    "Diskon (${order.discountValue}%)"
                else -> "Diskon"
            }
            sb.appendLine(twoColumnText(discountLabel, "-${formatPrice(discountAmt)}", charWidth))
        }

        // -- Tax --
        val taxAmt = BigDecimal(order.taxAmount)
        if (taxAmt.compareTo(BigDecimal.ZERO) > 0) {
            sb.appendLine(twoColumnText("Pajak", formatPrice(taxAmt), charWidth))
        }

        // -- Total --
        sb.appendLine(twoColumnText("TOTAL", formatPrice(BigDecimal(order.totalAmount)), charWidth))

        sb.appendLine("-".repeat(charWidth))

        // -- Payments --
        for (payment in order.payments) {
            val amount = BigDecimal(payment.amount)
            if (amount.compareTo(BigDecimal.ZERO) <= 0) continue
            val methodLabel = formatPaymentMethodStr(payment.paymentMethod)
            sb.appendLine(twoColumnText(methodLabel, formatPrice(amount), charWidth))
        }

        // -- Change --
        val totalChange = order.payments
            .filter { it.changeAmount != null }
            .fold(BigDecimal.ZERO) { acc, p -> acc.add(BigDecimal(p.changeAmount!!)) }
        if (totalChange.compareTo(BigDecimal.ZERO) > 0) {
            sb.appendLine(twoColumnText("Kembalian", formatPrice(totalChange), charWidth))
        }

        sb.appendLine("-".repeat(charWidth))

        // -- Footer --
        sb.appendLine(centerText("Terima Kasih", charWidth))

        return sb.toString()
    }

    /** Helper: append a single order item to plain-text builder. */
    private fun appendItemText(sb: StringBuilder, item: OrderItemResponse, charWidth: Int) {
        val qty = "${item.quantity}x"
        val price = formatPrice(BigDecimal(item.subtotal))
        val leftPart = "$qty Item"
        sb.appendLine(twoColumnText(leftPart, price, charWidth))

        // Unit price (if qty > 1, show per-unit)
        if (item.quantity > 1) {
            sb.appendLine("  @ ${formatPrice(BigDecimal(item.unitPrice))}")
        }

        // Modifiers (indented)
        for (modifier in item.modifiers) {
            val modPrice = BigDecimal(modifier.unitPrice).multiply(BigDecimal(modifier.quantity))
            val modText = if (modifier.quantity > 1) {
                "  + Modifier x${modifier.quantity}"
            } else {
                "  + Modifier"
            }
            if (modPrice.compareTo(BigDecimal.ZERO) != 0) {
                sb.appendLine(twoColumnText(modText, "+${formatPrice(modPrice)}", charWidth))
            } else {
                sb.appendLine(modText)
            }
        }

        // Item notes
        if (!item.notes.isNullOrBlank()) {
            sb.appendLine("  *${item.notes}")
        }
    }

    /** Two-column plain-text line: left-aligned label, right-aligned value. */
    private fun twoColumnText(left: String, right: String, width: Int): String {
        val available = width - right.length
        val paddedLeft = if (left.length > available) {
            left.substring(0, available)
        } else {
            left.padEnd(available)
        }
        return paddedLeft + right
    }

    /** Center text within the given width. */
    private fun centerText(text: String, width: Int): String {
        if (text.length >= width) return text
        val padding = (width - text.length) / 2
        return " ".repeat(padding) + text
    }
}
