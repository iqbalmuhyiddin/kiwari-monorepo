package com.kiwari.pos.data.repository

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.kiwari.pos.util.printer.EscPosCommands
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.printerDataStore: DataStore<Preferences> by preferencesDataStore(name = "printer_preferences")

data class PrinterPreferences(
    val printerAddress: String = "",
    val printerName: String = "",
    val paperWidth: Int = EscPosCommands.WIDTH_58MM,
    val autoPrintEnabled: Boolean = false,
    val outletName: String = ""
)

@Singleton
class PrinterPreferencesRepository @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        private val KEY_PRINTER_ADDRESS = stringPreferencesKey("printer_address")
        private val KEY_PRINTER_NAME = stringPreferencesKey("printer_name")
        private val KEY_PAPER_WIDTH = intPreferencesKey("paper_width")
        private val KEY_AUTO_PRINT = booleanPreferencesKey("auto_print_enabled")
        private val KEY_OUTLET_NAME = stringPreferencesKey("outlet_name")
    }

    val preferences: Flow<PrinterPreferences> = context.printerDataStore.data.map { prefs ->
        PrinterPreferences(
            printerAddress = prefs[KEY_PRINTER_ADDRESS] ?: "",
            printerName = prefs[KEY_PRINTER_NAME] ?: "",
            paperWidth = prefs[KEY_PAPER_WIDTH] ?: EscPosCommands.WIDTH_58MM,
            autoPrintEnabled = prefs[KEY_AUTO_PRINT] ?: false,
            outletName = prefs[KEY_OUTLET_NAME] ?: ""
        )
    }

    suspend fun setPrinter(address: String, name: String) {
        context.printerDataStore.edit { prefs ->
            prefs[KEY_PRINTER_ADDRESS] = address
            prefs[KEY_PRINTER_NAME] = name
        }
    }

    suspend fun clearPrinter() {
        context.printerDataStore.edit { prefs ->
            prefs[KEY_PRINTER_ADDRESS] = ""
            prefs[KEY_PRINTER_NAME] = ""
        }
    }

    suspend fun setPaperWidth(width: Int) {
        context.printerDataStore.edit { prefs ->
            prefs[KEY_PAPER_WIDTH] = width
        }
    }

    suspend fun setAutoPrint(enabled: Boolean) {
        context.printerDataStore.edit { prefs ->
            prefs[KEY_AUTO_PRINT] = enabled
        }
    }

    suspend fun setOutletName(name: String) {
        context.printerDataStore.edit { prefs ->
            prefs[KEY_OUTLET_NAME] = name
        }
    }
}
