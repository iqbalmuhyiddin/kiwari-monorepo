package com.kiwari.pos.util.printer

/**
 * ESC/POS command constants for thermal receipt printers.
 * Supports 58mm (32 char) and 80mm (48 char) paper widths.
 */
object EscPosCommands {

    // Initialize printer
    val INIT = byteArrayOf(0x1B, 0x40)

    // Line feed
    val LF = byteArrayOf(0x0A)

    // Cut paper (partial cut)
    val CUT = byteArrayOf(0x1D, 0x56, 0x01)

    // Feed n lines before cut
    fun feedLines(n: Int): ByteArray = byteArrayOf(0x1B, 0x64, n.toByte())

    // Text alignment
    val ALIGN_LEFT = byteArrayOf(0x1B, 0x61, 0x00)
    val ALIGN_CENTER = byteArrayOf(0x1B, 0x61, 0x01)
    val ALIGN_RIGHT = byteArrayOf(0x1B, 0x61, 0x02)

    // Bold
    val BOLD_ON = byteArrayOf(0x1B, 0x45, 0x01)
    val BOLD_OFF = byteArrayOf(0x1B, 0x45, 0x00)

    // Double height
    val DOUBLE_HEIGHT_ON = byteArrayOf(0x1B, 0x21, 0x10)
    val DOUBLE_HEIGHT_OFF = byteArrayOf(0x1B, 0x21, 0x00)

    // Double height + bold combined
    val DOUBLE_HEIGHT_BOLD_ON = byteArrayOf(0x1B, 0x21, 0x18)

    // Paper widths in characters
    const val WIDTH_58MM = 32
    const val WIDTH_80MM = 48

    /** Build a separator line of dashes for the given paper width. */
    fun separator(width: Int): ByteArray {
        return "-".repeat(width).toByteArray(Charsets.UTF_8) + LF
    }

    /** Format a two-column line: left-aligned label, right-aligned value. */
    fun twoColumnLine(left: String, right: String, width: Int): ByteArray {
        val available = width - right.length
        val paddedLeft = if (left.length > available) {
            left.substring(0, available)
        } else {
            left.padEnd(available)
        }
        return (paddedLeft + right).toByteArray(Charsets.UTF_8) + LF
    }

    /** Encode text as bytes + line feed. */
    fun textLine(text: String): ByteArray {
        return text.toByteArray(Charsets.UTF_8) + LF
    }

    /** Encode text as bytes (no line feed). */
    fun text(text: String): ByteArray {
        return text.toByteArray(Charsets.UTF_8)
    }
}
