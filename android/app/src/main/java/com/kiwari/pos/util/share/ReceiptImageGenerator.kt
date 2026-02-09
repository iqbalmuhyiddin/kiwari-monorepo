package com.kiwari.pos.util.share

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Typeface
import java.io.File
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Generates a PNG image from plain-text receipt/bill content.
 * Used for sharing via Android share intent.
 */
@Singleton
class ReceiptImageGenerator @Inject constructor() {

    fun generateImage(text: String, context: Context): File {
        val paint = Paint().apply {
            typeface = Typeface.MONOSPACE
            textSize = 28f
            color = Color.BLACK
            isAntiAlias = true
        }

        val lines = text.split("\n")
        val lineHeight = paint.fontSpacing
        val maxWidth = lines.maxOf { paint.measureText(it) }.toInt() + 40
        val height = (lineHeight * lines.size + 40).toInt()

        val bitmap = Bitmap.createBitmap(maxWidth, height, Bitmap.Config.ARGB_8888)
        val canvas = Canvas(bitmap)
        canvas.drawColor(Color.WHITE)

        lines.forEachIndexed { index, line ->
            canvas.drawText(line, 20f, 20f + lineHeight * (index + 1), paint)
        }

        // Clean up old receipt images
        context.cacheDir.listFiles { _, name -> name.startsWith("receipt_") && name.endsWith(".png") }
            ?.forEach { it.delete() }

        val file = File(context.cacheDir, "receipt_${System.currentTimeMillis()}.png")
        file.outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        bitmap.recycle()
        return file
    }
}
