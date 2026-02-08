package com.kiwari.pos.ui.settings

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.kiwari.pos.data.repository.PrinterPreferencesRepository
import com.kiwari.pos.util.printer.BluetoothPrinterDevice
import com.kiwari.pos.util.printer.BluetoothPrinterManager
import com.kiwari.pos.util.printer.EscPosCommands
import com.kiwari.pos.util.printer.PrinterConnectionState
import com.kiwari.pos.util.printer.PrinterService
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class PrinterSettingsUiState(
    val pairedDevices: List<BluetoothPrinterDevice> = emptyList(),
    val selectedAddress: String = "",
    val selectedName: String = "",
    val connectionState: PrinterConnectionState = PrinterConnectionState.DISCONNECTED,
    val paperWidth: Int = EscPosCommands.WIDTH_58MM,
    val autoPrintEnabled: Boolean = false,
    val outletName: String = "",
    val needsBluetoothPermission: Boolean = false,
    val bluetoothNotAvailable: Boolean = false,
    val testPrintResult: String? = null,
    val isTestPrinting: Boolean = false
)

@HiltViewModel
class PrinterSettingsViewModel @Inject constructor(
    private val printerManager: BluetoothPrinterManager,
    private val printerPrefs: PrinterPreferencesRepository,
    private val printerService: PrinterService
) : ViewModel() {

    private val _uiState = MutableStateFlow(PrinterSettingsUiState())
    val uiState: StateFlow<PrinterSettingsUiState> = _uiState.asStateFlow()

    private var outletNameSaveJob: Job? = null

    init {
        loadPreferences()
        observeConnectionState()
    }

    private fun loadPreferences() {
        viewModelScope.launch {
            printerPrefs.preferences.collect { prefs ->
                _uiState.update {
                    it.copy(
                        selectedAddress = prefs.printerAddress,
                        selectedName = prefs.printerName,
                        paperWidth = prefs.paperWidth,
                        autoPrintEnabled = prefs.autoPrintEnabled,
                        outletName = prefs.outletName
                    )
                }
            }
        }
    }

    private fun observeConnectionState() {
        viewModelScope.launch {
            printerManager.connectionState.collect { state ->
                _uiState.update { it.copy(connectionState = state) }
            }
        }
    }

    /**
     * Load paired Bluetooth devices.
     * Call after permission is granted or on screen entry.
     */
    fun loadPairedDevices() {
        if (!printerManager.hasBluetoothPermission()) {
            _uiState.update { it.copy(needsBluetoothPermission = true) }
            return
        }
        if (!printerManager.isBluetoothAvailable()) {
            _uiState.update { it.copy(bluetoothNotAvailable = true) }
            return
        }
        val devices = printerManager.getPairedDevices()
        _uiState.update {
            it.copy(
                pairedDevices = devices,
                needsBluetoothPermission = false,
                bluetoothNotAvailable = false
            )
        }
    }

    /** Called after runtime permission result. */
    fun onPermissionResult(granted: Boolean) {
        if (granted) {
            _uiState.update { it.copy(needsBluetoothPermission = false) }
            loadPairedDevices()
        }
    }

    fun onSelectDevice(device: BluetoothPrinterDevice) {
        viewModelScope.launch {
            printerPrefs.setPrinter(device.address, device.name)
            // Also try to connect
            printerManager.connect(device.address)
        }
    }

    fun onDeselectDevice() {
        viewModelScope.launch {
            printerManager.disconnect()
            printerPrefs.clearPrinter()
        }
    }

    fun onPaperWidthChanged(width: Int) {
        viewModelScope.launch {
            printerPrefs.setPaperWidth(width)
        }
    }

    fun onAutoPrintChanged(enabled: Boolean) {
        viewModelScope.launch {
            printerPrefs.setAutoPrint(enabled)
        }
    }

    fun onOutletNameChanged(name: String) {
        // Update UI state immediately
        _uiState.update { it.copy(outletName = name) }

        // Debounce DataStore write
        outletNameSaveJob?.cancel()
        outletNameSaveJob = viewModelScope.launch {
            delay(500)
            printerPrefs.setOutletName(name)
        }
    }

    fun onTestPrint() {
        val state = _uiState.value
        if (state.selectedAddress.isBlank()) {
            _uiState.update { it.copy(testPrintResult = "Pilih printer terlebih dahulu") }
            return
        }
        viewModelScope.launch {
            _uiState.update { it.copy(isTestPrinting = true, testPrintResult = null) }
            val success = printerService.printTestPage(
                printerAddress = state.selectedAddress,
                paperWidth = state.paperWidth,
                outletName = state.outletName
            )
            _uiState.update {
                it.copy(
                    isTestPrinting = false,
                    testPrintResult = if (success) "Test print berhasil!" else "Test print gagal. Periksa koneksi printer."
                )
            }
        }
    }

    fun onDismissTestResult() {
        _uiState.update { it.copy(testPrintResult = null) }
    }
}
