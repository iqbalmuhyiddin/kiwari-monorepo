package com.kiwari.pos.util.printer

import android.util.Log
import com.kiwari.pos.data.repository.PrinterPreferencesRepository
import kotlinx.coroutines.flow.first
import javax.inject.Inject
import javax.inject.Singleton

/**
 * High-level service for printing receipts and kitchen tickets.
 * ViewModels call this; it handles preferences check, connection, formatting, and sending.
 */
@Singleton
class PrinterService @Inject constructor(
    private val printerManager: BluetoothPrinterManager,
    private val printerPrefs: PrinterPreferencesRepository
) {
    companion object {
        private const val TAG = "PrinterService"
    }

    /**
     * Print a receipt if auto-print is enabled and a printer is configured.
     * Safe to call fire-and-forget — logs errors but does not throw.
     */
    suspend fun printReceiptIfEnabled(data: ReceiptData) {
        try {
            val prefs = printerPrefs.preferences.first()
            if (!prefs.autoPrintEnabled || prefs.printerAddress.isBlank()) {
                Log.d(TAG, "Auto-print disabled or no printer configured — skipping")
                return
            }

            val receiptData = data.copy(
                outletName = prefs.outletName.ifBlank { data.outletName }
            )

            if (!ensureConnected(prefs.printerAddress)) return

            val bytes = ReceiptFormatter.formatReceipt(receiptData, prefs.paperWidth)
            val success = printerManager.send(bytes)
            if (!success) {
                Log.e(TAG, "Failed to send receipt data")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error printing receipt: ${e.message}")
        }
    }

    /**
     * Print a kitchen ticket if auto-print is enabled and a printer is configured.
     * Safe to call fire-and-forget — logs errors but does not throw.
     */
    suspend fun printKitchenTicketIfEnabled(data: ReceiptData) {
        try {
            val prefs = printerPrefs.preferences.first()
            if (!prefs.autoPrintEnabled || prefs.printerAddress.isBlank()) {
                Log.d(TAG, "Auto-print disabled or no printer configured — skipping")
                return
            }

            if (!ensureConnected(prefs.printerAddress)) return

            val bytes = ReceiptFormatter.formatKitchenTicket(data, prefs.paperWidth)
            val success = printerManager.send(bytes)
            if (!success) {
                Log.e(TAG, "Failed to send kitchen ticket data")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error printing kitchen ticket: ${e.message}")
        }
    }

    /**
     * Print a test receipt to verify printer connection.
     * Returns true if successful.
     */
    suspend fun printTestPage(printerAddress: String, paperWidth: Int, outletName: String): Boolean {
        return try {
            if (!ensureConnected(printerAddress)) return false

            val cmd = EscPosCommands
            val buf = mutableListOf<ByteArray>()
            buf += cmd.INIT
            buf += cmd.ALIGN_CENTER
            buf += cmd.DOUBLE_HEIGHT_BOLD_ON
            buf += cmd.textLine(outletName.ifBlank { "Test Print" })
            buf += cmd.DOUBLE_HEIGHT_OFF
            buf += cmd.BOLD_OFF
            buf += cmd.LF
            buf += cmd.ALIGN_LEFT
            buf += cmd.separator(paperWidth)
            buf += cmd.textLine("Printer terhubung!")
            buf += cmd.textLine("Lebar: ${if (paperWidth == EscPosCommands.WIDTH_58MM) "58mm" else "80mm"}")
            buf += cmd.separator(paperWidth)
            buf += cmd.ALIGN_CENTER
            buf += cmd.textLine("Kiwari POS")
            buf += cmd.feedLines(3)
            buf += cmd.CUT

            val data = buf.merge()
            printerManager.send(data)
        } catch (e: Exception) {
            Log.e(TAG, "Test print failed: ${e.message}")
            false
        }
    }

    private suspend fun ensureConnected(address: String): Boolean {
        if (printerManager.isConnected()) return true
        val connected = printerManager.connect(address)
        if (!connected) {
            Log.e(TAG, "Could not connect to printer at $address")
        }
        return connected
    }
}
