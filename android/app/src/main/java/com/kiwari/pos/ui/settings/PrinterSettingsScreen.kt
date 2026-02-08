package com.kiwari.pos.ui.settings

import android.Manifest
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.util.printer.EscPosCommands
import com.kiwari.pos.util.printer.PrinterConnectionState

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PrinterSettingsScreen(
    viewModel: PrinterSettingsViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()
    val snackbarHostState = remember { SnackbarHostState() }

    // Bluetooth permission launcher (API 31+)
    val permissionLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.RequestPermission()
    ) { granted ->
        viewModel.onPermissionResult(granted)
    }

    // Load devices on entry
    LaunchedEffect(Unit) {
        viewModel.loadPairedDevices()
    }

    // Request permission if needed
    LaunchedEffect(uiState.needsBluetoothPermission) {
        if (uiState.needsBluetoothPermission && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            permissionLauncher.launch(Manifest.permission.BLUETOOTH_CONNECT)
        }
    }

    // Show test print result
    LaunchedEffect(uiState.testPrintResult) {
        uiState.testPrintResult?.let { msg ->
            snackbarHostState.showSnackbar(msg)
            viewModel.onDismissTestResult()
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Pengaturan Printer") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = "Kembali"
                        )
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { paddingValues ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            // Connection status
            item {
                Spacer(modifier = Modifier.height(8.dp))
                ConnectionStatusCard(uiState)
            }

            // Outlet name
            item {
                SectionTitle("Nama Outlet")
                OutlinedTextField(
                    value = uiState.outletName,
                    onValueChange = viewModel::onOutletNameChanged,
                    modifier = Modifier.fillMaxWidth(),
                    placeholder = { Text("Nama outlet untuk struk") },
                    singleLine = true,
                    shape = MaterialTheme.shapes.small
                )
            }

            // Paper width selector
            item {
                SectionTitle("Lebar Kertas")
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(
                        selected = uiState.paperWidth == EscPosCommands.WIDTH_58MM,
                        onClick = { viewModel.onPaperWidthChanged(EscPosCommands.WIDTH_58MM) },
                        label = { Text("58mm") },
                        leadingIcon = if (uiState.paperWidth == EscPosCommands.WIDTH_58MM) {
                            { Icon(Icons.Default.Check, contentDescription = null, modifier = Modifier.size(16.dp)) }
                        } else null
                    )
                    FilterChip(
                        selected = uiState.paperWidth == EscPosCommands.WIDTH_80MM,
                        onClick = { viewModel.onPaperWidthChanged(EscPosCommands.WIDTH_80MM) },
                        label = { Text("80mm") },
                        leadingIcon = if (uiState.paperWidth == EscPosCommands.WIDTH_80MM) {
                            { Icon(Icons.Default.Check, contentDescription = null, modifier = Modifier.size(16.dp)) }
                        } else null
                    )
                }
            }

            // Auto-print toggle
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "Auto-Print",
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.SemiBold
                        )
                        Text(
                            text = "Cetak struk otomatis setelah pembayaran",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    Switch(
                        checked = uiState.autoPrintEnabled,
                        onCheckedChange = viewModel::onAutoPrintChanged
                    )
                }
            }

            // Paired devices
            item {
                SectionTitle("Perangkat Bluetooth")
                if (uiState.bluetoothNotAvailable) {
                    Text(
                        text = "Bluetooth tidak tersedia atau tidak aktif",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }

            if (uiState.pairedDevices.isEmpty() && !uiState.bluetoothNotAvailable) {
                item {
                    Text(
                        text = "Tidak ada perangkat Bluetooth terpasangkan. Pasangkan printer melalui Pengaturan Bluetooth perangkat.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            items(uiState.pairedDevices, key = { it.address }) { device ->
                val isSelected = device.address == uiState.selectedAddress
                Card(
                    modifier = Modifier
                        .fillMaxWidth()
                        .then(
                            if (isSelected) {
                                Modifier.border(
                                    width = 2.dp,
                                    color = MaterialTheme.colorScheme.primary,
                                    shape = MaterialTheme.shapes.medium
                                )
                            } else Modifier
                        )
                        .clickable {
                            if (isSelected) {
                                viewModel.onDeselectDevice()
                            } else {
                                viewModel.onSelectDevice(device)
                            }
                        },
                    colors = CardDefaults.cardColors(
                        containerColor = if (isSelected) {
                            MaterialTheme.colorScheme.primaryContainer
                        } else {
                            MaterialTheme.colorScheme.surface
                        }
                    ),
                    shape = MaterialTheme.shapes.medium
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = device.name,
                                style = MaterialTheme.typography.bodyLarge,
                                fontWeight = FontWeight.Medium
                            )
                            Text(
                                text = device.address,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                        if (isSelected) {
                            Icon(
                                imageVector = Icons.Default.Check,
                                contentDescription = "Terpilih",
                                tint = MaterialTheme.colorScheme.primary
                            )
                        }
                    }
                }
            }

            // Test print button
            item {
                Spacer(modifier = Modifier.height(8.dp))
                Button(
                    onClick = viewModel::onTestPrint,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = uiState.selectedAddress.isNotBlank() && !uiState.isTestPrinting,
                    shape = MaterialTheme.shapes.small
                ) {
                    if (uiState.isTestPrinting) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(20.dp),
                            strokeWidth = 2.dp,
                            color = MaterialTheme.colorScheme.onPrimary
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                    }
                    Text(if (uiState.isTestPrinting) "Mencetak..." else "Test Print")
                }
                Spacer(modifier = Modifier.height(16.dp))
            }
        }
    }
}

@Composable
private fun ConnectionStatusCard(uiState: PrinterSettingsUiState) {
    val (statusText, statusColor) = when (uiState.connectionState) {
        PrinterConnectionState.CONNECTED -> "Terhubung" to MaterialTheme.colorScheme.primary
        PrinterConnectionState.CONNECTING -> "Menghubungkan..." to MaterialTheme.colorScheme.tertiary
        PrinterConnectionState.ERROR -> "Gagal terhubung" to MaterialTheme.colorScheme.error
        PrinterConnectionState.DISCONNECTED -> {
            if (uiState.selectedAddress.isNotBlank()) {
                "Terputus" to MaterialTheme.colorScheme.onSurfaceVariant
            } else {
                "Belum dipilih" to MaterialTheme.colorScheme.onSurfaceVariant
            }
        }
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        ),
        shape = MaterialTheme.shapes.medium
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text(
                    text = "Status Printer",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold
                )
                if (uiState.selectedName.isNotBlank()) {
                    Text(
                        text = uiState.selectedName,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (uiState.connectionState == PrinterConnectionState.CONNECTING) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(16.dp),
                        strokeWidth = 2.dp
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                }
                Box(
                    modifier = Modifier
                        .size(10.dp)
                        .background(
                            color = statusColor,
                            shape = MaterialTheme.shapes.extraSmall
                        )
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = statusText,
                    style = MaterialTheme.typography.bodyMedium,
                    color = statusColor,
                    fontWeight = FontWeight.Medium
                )
            }
        }
    }
}

@Composable
private fun SectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleSmall,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.padding(bottom = 4.dp)
    )
}
