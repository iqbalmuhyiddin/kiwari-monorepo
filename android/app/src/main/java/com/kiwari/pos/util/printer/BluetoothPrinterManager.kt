package com.kiwari.pos.util.printer

import android.Manifest
import android.annotation.SuppressLint
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothSocket
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.util.Log
import androidx.core.content.ContextCompat
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.io.IOException
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

enum class PrinterConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    ERROR
}

data class BluetoothPrinterDevice(
    val name: String,
    val address: String // MAC address
)

/**
 * Manages Bluetooth connection to a thermal printer.
 * Singleton — shared across the app lifecycle.
 */
@Singleton
class BluetoothPrinterManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        private const val TAG = "BluetoothPrinter"
        // Standard Serial Port Profile UUID
        private val SPP_UUID: UUID = UUID.fromString("00001101-0000-1000-8000-00805F9B34FB")
    }

    private val bluetoothManager: BluetoothManager? =
        context.getSystemService(Context.BLUETOOTH_SERVICE) as? BluetoothManager
    private val bluetoothAdapter: BluetoothAdapter? = bluetoothManager?.adapter

    private var socket: BluetoothSocket? = null
    private val socketMutex = Mutex()

    private val _connectionState = MutableStateFlow(PrinterConnectionState.DISCONNECTED)
    val connectionState: StateFlow<PrinterConnectionState> = _connectionState.asStateFlow()

    private val _connectedDeviceName = MutableStateFlow<String?>(null)
    val connectedDeviceName: StateFlow<String?> = _connectedDeviceName.asStateFlow()

    /** Check if Bluetooth is available and enabled. */
    fun isBluetoothAvailable(): Boolean = bluetoothAdapter?.isEnabled == true

    /** Check if BLUETOOTH_CONNECT permission is granted (needed on API 31+). */
    fun hasBluetoothPermission(): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            ContextCompat.checkSelfPermission(
                context, Manifest.permission.BLUETOOTH_CONNECT
            ) == PackageManager.PERMISSION_GRANTED
        } else {
            true // Pre-Android 12 doesn't need runtime permission
        }
    }

    /**
     * Get list of paired Bluetooth devices.
     * Returns empty list if permission not granted or Bluetooth not available.
     */
    @SuppressLint("MissingPermission")
    fun getPairedDevices(): List<BluetoothPrinterDevice> {
        if (!hasBluetoothPermission() || !isBluetoothAvailable()) return emptyList()

        return bluetoothAdapter?.bondedDevices?.map { device ->
            BluetoothPrinterDevice(
                name = device.name ?: "Unknown",
                address = device.address
            )
        } ?: emptyList()
    }

    /**
     * Connect to a Bluetooth printer by MAC address.
     * Must be called from a coroutine (runs on Dispatchers.IO).
     */
    @SuppressLint("MissingPermission")
    suspend fun connect(address: String): Boolean = withContext(Dispatchers.IO) {
        socketMutex.withLock {
            try {
                _connectionState.value = PrinterConnectionState.CONNECTING

                // Disconnect existing connection first
                disconnectInternal()

                if (!hasBluetoothPermission() || !isBluetoothAvailable()) {
                    Log.e(TAG, "Bluetooth not available or permission not granted")
                    _connectionState.value = PrinterConnectionState.ERROR
                    return@withContext false
                }

                val device: BluetoothDevice = bluetoothAdapter!!.getRemoteDevice(address)
                socket = device.createRfcommSocketToServiceRecord(SPP_UUID)
                socket!!.connect()

                _connectionState.value = PrinterConnectionState.CONNECTED
                _connectedDeviceName.value = device.name ?: address
                Log.d(TAG, "Connected to ${device.name} ($address)")
                true
            } catch (e: IOException) {
                Log.e(TAG, "Connection failed: ${e.message}")
                disconnectInternal()
                _connectionState.value = PrinterConnectionState.ERROR
                false
            } catch (e: IllegalArgumentException) {
                Log.e(TAG, "Invalid MAC address: $address")
                _connectionState.value = PrinterConnectionState.ERROR
                false
            }
        }
    }

    /**
     * Send raw bytes to the connected printer.
     * Returns true on success, false on failure.
     */
    suspend fun send(data: ByteArray): Boolean = withContext(Dispatchers.IO) {
        socketMutex.withLock {
            try {
                val outputStream = socket?.outputStream
                if (outputStream == null) {
                    Log.e(TAG, "Not connected — cannot send data")
                    _connectionState.value = PrinterConnectionState.DISCONNECTED
                    return@withContext false
                }
                outputStream.write(data)
                outputStream.flush()
                true
            } catch (e: IOException) {
                Log.e(TAG, "Send failed: ${e.message}")
                _connectionState.value = PrinterConnectionState.ERROR
                disconnectInternal()
                false
            }
        }
    }

    /** Disconnect from the printer. */
    suspend fun disconnect() = withContext(Dispatchers.IO) {
        socketMutex.withLock {
            disconnectInternal()
        }
    }

    private fun disconnectInternal() {
        try {
            socket?.close()
        } catch (e: IOException) {
            Log.w(TAG, "Error closing socket: ${e.message}")
        }
        socket = null
        _connectionState.value = PrinterConnectionState.DISCONNECTED
        _connectedDeviceName.value = null
    }

    /** Check if currently connected. */
    fun isConnected(): Boolean = _connectionState.value == PrinterConnectionState.CONNECTED
}
