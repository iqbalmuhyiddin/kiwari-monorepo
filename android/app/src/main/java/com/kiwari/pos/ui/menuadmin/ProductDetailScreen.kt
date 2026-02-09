package com.kiwari.pos.ui.menuadmin

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.MenuAnchorType
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedCard
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.data.model.Product
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ProductDetailScreen(
    viewModel: ProductDetailViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {},
    onProductCreated: (productId: String) -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()

    // Navigate after successful create
    LaunchedEffect(uiState.savedProductId) {
        val savedId = uiState.savedProductId
        if (savedId != null) {
            viewModel.clearSavedProductId()
            onProductCreated(savedId)
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Top bar
            ProductDetailTopBar(
                title = if (uiState.isCreateMode) "Produk Baru" else uiState.name,
                onNavigateBack = onNavigateBack
            )

            when {
                uiState.isLoading -> {
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .fillMaxWidth(),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator(
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }

                uiState.errorMessage != null -> {
                    Box(
                        modifier = Modifier
                            .weight(1f)
                            .fillMaxWidth(),
                        contentAlignment = Alignment.Center
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally) {
                            Text(
                                text = uiState.errorMessage ?: "Terjadi kesalahan",
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.error
                            )
                        }
                    }
                }

                else -> {
                    ProductDetailContent(
                        uiState = uiState,
                        onNameChanged = viewModel::onNameChanged,
                        onBasePriceChanged = viewModel::onBasePriceChanged,
                        onCategorySelected = viewModel::onCategorySelected,
                        onStationSelected = viewModel::onStationSelected,
                        onDescriptionChanged = viewModel::onDescriptionChanged,
                        onPreparationTimeChanged = viewModel::onPreparationTimeChanged,
                        onIsComboChanged = viewModel::onIsComboChanged,
                        onSave = viewModel::saveProduct,
                        onShowDeactivateDialog = viewModel::showDeactivateDialog,
                        onShowSheet = viewModel::showSheet,
                        onDeleteVariantGroup = viewModel::deleteVariantGroup,
                        onDeleteVariant = viewModel::deleteVariant,
                        onDeleteModifierGroup = viewModel::deleteModifierGroup,
                        onDeleteModifier = viewModel::deleteModifier,
                        onRemoveComboItem = viewModel::removeComboItem,
                        modifier = Modifier.weight(1f)
                    )
                }
            }
        }

        // Deactivating overlay
        if (uiState.isDeactivating) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Color.Black.copy(alpha = 0.3f))
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        onClick = {}
                    ),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator(
                    color = MaterialTheme.colorScheme.primary
                )
            }
        }
    }

    // Deactivate confirmation dialog
    if (uiState.showDeactivateDialog) {
        AlertDialog(
            onDismissRequest = viewModel::dismissDeactivateDialog,
            title = { Text("Nonaktifkan Produk") },
            text = {
                Text("Produk \"${uiState.name}\" akan dinonaktifkan. Produk tidak akan muncul di menu.")
            },
            confirmButton = {
                TextButton(onClick = viewModel::deactivateProduct) {
                    Text(
                        text = "Nonaktifkan",
                        color = MaterialTheme.colorScheme.error
                    )
                }
            },
            dismissButton = {
                TextButton(onClick = viewModel::dismissDeactivateDialog) {
                    Text("Batal")
                }
            }
        )
    }

    // Bottom sheets
    val activeSheet = uiState.activeSheet
    if (activeSheet != null) {
        val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
        ModalBottomSheet(
            onDismissRequest = { if (!uiState.isSaving) viewModel.dismissSheet() },
            sheetState = sheetState
        ) {
            when (activeSheet) {
                is ProductSheet.EditVariantGroup -> {
                    VariantGroupSheetContent(
                        group = activeSheet.group,
                        isSaving = uiState.isSaving,
                        saveError = uiState.saveError,
                        onSave = { name, isRequired -> viewModel.saveVariantGroup(name, isRequired) },
                        onDelete = if (activeSheet.group != null) {
                            { viewModel.deleteVariantGroup(activeSheet.group.id) }
                        } else null
                    )
                }
                is ProductSheet.EditVariant -> {
                    VariantSheetContent(
                        variant = activeSheet.variant,
                        isSaving = uiState.isSaving,
                        saveError = uiState.saveError,
                        onSave = { name, priceAdj ->
                            viewModel.saveVariant(activeSheet.groupId, name, priceAdj)
                        },
                        onDelete = if (activeSheet.variant != null) {
                            { viewModel.deleteVariant(activeSheet.groupId, activeSheet.variant.id) }
                        } else null
                    )
                }
                is ProductSheet.EditModifierGroup -> {
                    ModifierGroupSheetContent(
                        group = activeSheet.group,
                        isSaving = uiState.isSaving,
                        saveError = uiState.saveError,
                        onSave = { name, minSelect, maxSelect ->
                            viewModel.saveModifierGroup(name, minSelect, maxSelect)
                        },
                        onDelete = if (activeSheet.group != null) {
                            { viewModel.deleteModifierGroup(activeSheet.group.id) }
                        } else null
                    )
                }
                is ProductSheet.EditModifier -> {
                    ModifierSheetContent(
                        modifier = activeSheet.modifier,
                        isSaving = uiState.isSaving,
                        saveError = uiState.saveError,
                        onSave = { name, price ->
                            viewModel.saveModifier(activeSheet.groupId, name, price)
                        },
                        onDelete = if (activeSheet.modifier != null) {
                            { viewModel.deleteModifier(activeSheet.groupId, activeSheet.modifier.id) }
                        } else null
                    )
                }
                is ProductSheet.AddComboItem -> {
                    ComboItemSheetContent(
                        allProducts = uiState.allProducts,
                        isSaving = uiState.isSaving,
                        saveError = uiState.saveError,
                        onAdd = { productId, quantity ->
                            viewModel.addComboItem(productId, quantity)
                        }
                    )
                }
            }
        }
    }
}

// ── Top Bar ──

@Composable
private fun ProductDetailTopBar(
    title: String,
    onNavigateBack: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = 4.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        IconButton(onClick = onNavigateBack) {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "Kembali"
            )
        }
        Text(
            text = title,
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f)
        )
    }
}

// ── Main Content ──

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ProductDetailContent(
    uiState: ProductDetailUiState,
    onNameChanged: (String) -> Unit,
    onBasePriceChanged: (String) -> Unit,
    onCategorySelected: (String) -> Unit,
    onStationSelected: (String) -> Unit,
    onDescriptionChanged: (String) -> Unit,
    onPreparationTimeChanged: (String) -> Unit,
    onIsComboChanged: (Boolean) -> Unit,
    onSave: () -> Unit,
    onShowDeactivateDialog: () -> Unit,
    onShowSheet: (ProductSheet) -> Unit,
    onDeleteVariantGroup: (String) -> Unit,
    onDeleteVariant: (groupId: String, variantId: String) -> Unit,
    onDeleteModifierGroup: (String) -> Unit,
    onDeleteModifier: (groupId: String, modifierId: String) -> Unit,
    onRemoveComboItem: (comboItemId: String) -> Unit,
    modifier: Modifier = Modifier
) {
    val stationOptions = listOf("" to "Tidak ada", "GRILL" to "Grill", "BEVERAGE" to "Beverage", "RICE" to "Rice", "DESSERT" to "Dessert")
    var variantSectionExpanded by remember { mutableStateOf(true) }
    var modifierSectionExpanded by remember { mutableStateOf(true) }
    var comboSectionExpanded by remember { mutableStateOf(true) }

    Column(modifier = modifier.fillMaxSize()) {
        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth(),
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // Inactive badge
            if (!uiState.isCreateMode && !uiState.isActive) {
                item {
                    Text(
                        text = "Nonaktif",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.error,
                        fontWeight = FontWeight.Bold
                    )
                }
            }

            // Name
            item {
                OutlinedTextField(
                    value = uiState.name,
                    onValueChange = onNameChanged,
                    label = { Text("Nama produk") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !uiState.isSaving
                )
            }

            // Base price
            item {
                OutlinedTextField(
                    value = uiState.basePrice,
                    onValueChange = onBasePriceChanged,
                    label = { Text("Harga dasar") },
                    singleLine = true,
                    prefix = { Text("Rp") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !uiState.isSaving
                )
            }

            // Category dropdown
            item {
                var categoryExpanded by remember { mutableStateOf(false) }
                val selectedCategory = uiState.categories.find { it.id == uiState.categoryId }

                ExposedDropdownMenuBox(
                    expanded = categoryExpanded,
                    onExpandedChange = { categoryExpanded = it }
                ) {
                    OutlinedTextField(
                        value = selectedCategory?.name ?: "",
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Kategori") },
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = categoryExpanded) },
                        modifier = Modifier
                            .fillMaxWidth()
                            .menuAnchor(MenuAnchorType.PrimaryNotEditable),
                        enabled = !uiState.isSaving
                    )
                    ExposedDropdownMenu(
                        expanded = categoryExpanded,
                        onDismissRequest = { categoryExpanded = false }
                    ) {
                        uiState.categories.forEach { category ->
                            DropdownMenuItem(
                                text = { Text(category.name) },
                                onClick = {
                                    onCategorySelected(category.id)
                                    categoryExpanded = false
                                }
                            )
                        }
                    }
                }
            }

            // Station dropdown
            item {
                var stationExpanded by remember { mutableStateOf(false) }
                val selectedStation = stationOptions.find { it.first == uiState.station }

                ExposedDropdownMenuBox(
                    expanded = stationExpanded,
                    onExpandedChange = { stationExpanded = it }
                ) {
                    OutlinedTextField(
                        value = selectedStation?.second ?: "Tidak ada",
                        onValueChange = {},
                        readOnly = true,
                        label = { Text("Station") },
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = stationExpanded) },
                        modifier = Modifier
                            .fillMaxWidth()
                            .menuAnchor(MenuAnchorType.PrimaryNotEditable),
                        enabled = !uiState.isSaving
                    )
                    ExposedDropdownMenu(
                        expanded = stationExpanded,
                        onDismissRequest = { stationExpanded = false }
                    ) {
                        stationOptions.forEach { (value, label) ->
                            DropdownMenuItem(
                                text = { Text(label) },
                                onClick = {
                                    onStationSelected(value)
                                    stationExpanded = false
                                }
                            )
                        }
                    }
                }
            }

            // Description
            item {
                OutlinedTextField(
                    value = uiState.description,
                    onValueChange = onDescriptionChanged,
                    label = { Text("Deskripsi (opsional)") },
                    modifier = Modifier.fillMaxWidth(),
                    minLines = 2,
                    maxLines = 4,
                    enabled = !uiState.isSaving
                )
            }

            // Preparation time
            item {
                OutlinedTextField(
                    value = uiState.preparationTime,
                    onValueChange = onPreparationTimeChanged,
                    label = { Text("Waktu persiapan (opsional)") },
                    singleLine = true,
                    suffix = { Text("menit") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !uiState.isSaving
                )
            }

            // Is combo toggle
            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = "Produk combo",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Switch(
                        checked = uiState.isCombo,
                        onCheckedChange = onIsComboChanged,
                        enabled = !uiState.isSaving && uiState.isCreateMode
                    )
                }
            }

            // ── Variant Groups Section (edit mode only) ──
            if (!uiState.isCreateMode) {
                item {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                }

                item {
                    CollapsibleSectionHeader(
                        title = "Varian",
                        isExpanded = variantSectionExpanded,
                        onToggle = { variantSectionExpanded = !variantSectionExpanded },
                        onAdd = { onShowSheet(ProductSheet.EditVariantGroup(null)) }
                    )
                }

                if (variantSectionExpanded) {
                    if (uiState.variantGroups.isEmpty()) {
                        item {
                            Text(
                                text = "Belum ada grup varian",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    } else {
                        uiState.variantGroups.forEach { groupWithVariants ->
                            item(key = "vg_${groupWithVariants.group.id}") {
                                VariantGroupCard(
                                    groupWithVariants = groupWithVariants,
                                    onEditGroup = {
                                        onShowSheet(ProductSheet.EditVariantGroup(groupWithVariants.group))
                                    },
                                    onDeleteGroup = { onDeleteVariantGroup(groupWithVariants.group.id) },
                                    onEditVariant = { variant ->
                                        onShowSheet(ProductSheet.EditVariant(groupWithVariants.group.id, variant))
                                    },
                                    onDeleteVariant = { variant ->
                                        onDeleteVariant(groupWithVariants.group.id, variant.id)
                                    },
                                    onAddVariant = {
                                        onShowSheet(ProductSheet.EditVariant(groupWithVariants.group.id, null))
                                    }
                                )
                            }
                        }
                    }
                }

                // ── Modifier Groups Section ──
                item {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                }

                item {
                    CollapsibleSectionHeader(
                        title = "Modifier",
                        isExpanded = modifierSectionExpanded,
                        onToggle = { modifierSectionExpanded = !modifierSectionExpanded },
                        onAdd = { onShowSheet(ProductSheet.EditModifierGroup(null)) }
                    )
                }

                if (modifierSectionExpanded) {
                    if (uiState.modifierGroups.isEmpty()) {
                        item {
                            Text(
                                text = "Belum ada grup modifier",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    } else {
                        uiState.modifierGroups.forEach { groupWithModifiers ->
                            item(key = "mg_${groupWithModifiers.group.id}") {
                                ModifierGroupCard(
                                    groupWithModifiers = groupWithModifiers,
                                    onEditGroup = {
                                        onShowSheet(ProductSheet.EditModifierGroup(groupWithModifiers.group))
                                    },
                                    onDeleteGroup = { onDeleteModifierGroup(groupWithModifiers.group.id) },
                                    onEditModifier = { mod ->
                                        onShowSheet(ProductSheet.EditModifier(groupWithModifiers.group.id, mod))
                                    },
                                    onDeleteModifier = { mod ->
                                        onDeleteModifier(groupWithModifiers.group.id, mod.id)
                                    },
                                    onAddModifier = {
                                        onShowSheet(ProductSheet.EditModifier(groupWithModifiers.group.id, null))
                                    }
                                )
                            }
                        }
                    }
                }

                // ── Combo Items Section (only if isCombo) ──
                if (uiState.isCombo) {
                    item {
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                    }

                    item {
                        CollapsibleSectionHeader(
                            title = "Combo Items",
                            isExpanded = comboSectionExpanded,
                            onToggle = { comboSectionExpanded = !comboSectionExpanded },
                            onAdd = { onShowSheet(ProductSheet.AddComboItem) }
                        )
                    }

                    if (comboSectionExpanded) {
                        if (uiState.comboItems.isEmpty()) {
                            item {
                                Text(
                                    text = "Belum ada item combo",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        } else {
                            uiState.comboItems.forEach { comboItemWithProduct ->
                                item(key = "ci_${comboItemWithProduct.comboItem.id}") {
                                    ComboItemRow(
                                        item = comboItemWithProduct,
                                        onDelete = { onRemoveComboItem(comboItemWithProduct.comboItem.id) }
                                    )
                                }
                            }
                        }
                    }
                }
            }

            // Bottom spacing
            item {
                Spacer(modifier = Modifier.height(8.dp))
            }
        }

        // Bottom actions
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(MaterialTheme.colorScheme.surface)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            // Save error
            if (uiState.saveError != null) {
                Text(
                    text = uiState.saveError,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error
                )
            }

            // Save button
            Button(
                onClick = onSave,
                modifier = Modifier.fillMaxWidth(),
                enabled = !uiState.isSaving && uiState.name.isNotBlank() && uiState.basePrice.isNotBlank() && uiState.categoryId.isNotBlank(),
                shape = MaterialTheme.shapes.medium
            ) {
                if (uiState.isSaving) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                } else {
                    Text("Simpan")
                }
            }

            // Deactivate button (edit mode only)
            if (!uiState.isCreateMode && uiState.isActive) {
                TextButton(
                    onClick = onShowDeactivateDialog,
                    modifier = Modifier.fillMaxWidth(),
                    colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
                ) {
                    Text("Nonaktifkan Produk")
                }
            }
        }
    }
}

// ── Collapsible Section Header ──

@Composable
private fun CollapsibleSectionHeader(
    title: String,
    isExpanded: Boolean,
    onToggle: () -> Unit,
    onAdd: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onToggle),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = if (isExpanded) Icons.Default.KeyboardArrowUp else Icons.Default.KeyboardArrowDown,
            contentDescription = if (isExpanded) "Tutup" else "Buka",
            modifier = Modifier.size(24.dp),
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.width(4.dp))
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.weight(1f)
        )
        IconButton(onClick = onAdd, modifier = Modifier.size(36.dp)) {
            Icon(
                imageVector = Icons.Default.Add,
                contentDescription = "Tambah $title",
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

// ── Variant Group Card ──

@Composable
private fun VariantGroupCard(
    groupWithVariants: VariantGroupWithVariants,
    onEditGroup: () -> Unit,
    onDeleteGroup: () -> Unit,
    onEditVariant: (com.kiwari.pos.data.model.Variant) -> Unit,
    onDeleteVariant: (com.kiwari.pos.data.model.Variant) -> Unit,
    onAddVariant: () -> Unit
) {
    val group = groupWithVariants.group
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            // Group header
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = group.name,
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        if (group.isRequired) {
                            Text(
                                text = "Wajib",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.primary,
                                fontWeight = FontWeight.Medium
                            )
                        }
                    }
                }
                IconButton(onClick = onEditGroup, modifier = Modifier.size(32.dp)) {
                    Icon(
                        imageVector = Icons.Default.Edit,
                        contentDescription = "Edit",
                        modifier = Modifier.size(16.dp)
                    )
                }
                IconButton(onClick = onDeleteGroup, modifier = Modifier.size(32.dp)) {
                    Icon(
                        imageVector = Icons.Default.Delete,
                        contentDescription = "Hapus",
                        tint = MaterialTheme.colorScheme.error,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }

            // Variants list
            if (groupWithVariants.variants.isNotEmpty()) {
                Spacer(modifier = Modifier.height(8.dp))
                groupWithVariants.variants.forEach { variant ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = variant.name,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            modifier = Modifier.weight(1f)
                        )
                        val adjustment = variant.priceAdjustment.toBigDecimalOrNull() ?: BigDecimal.ZERO
                        if (adjustment.compareTo(BigDecimal.ZERO) != 0) {
                            val prefix = if (adjustment > BigDecimal.ZERO) "+" else ""
                            Text(
                                text = "$prefix${formatPrice(adjustment)}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                        IconButton(
                            onClick = { onEditVariant(variant) },
                            modifier = Modifier.size(28.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Edit,
                                contentDescription = "Edit",
                                modifier = Modifier.size(14.dp)
                            )
                        }
                        IconButton(
                            onClick = { onDeleteVariant(variant) },
                            modifier = Modifier.size(28.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Delete,
                                contentDescription = "Hapus",
                                tint = MaterialTheme.colorScheme.error,
                                modifier = Modifier.size(14.dp)
                            )
                        }
                    }
                }
            }

            // Add variant button
            Spacer(modifier = Modifier.height(4.dp))
            TextButton(
                onClick = onAddVariant,
                contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp)
            ) {
                Icon(
                    imageVector = Icons.Default.Add,
                    contentDescription = null,
                    modifier = Modifier.size(16.dp)
                )
                Spacer(modifier = Modifier.width(4.dp))
                Text(
                    text = "Tambah Varian",
                    style = MaterialTheme.typography.labelMedium
                )
            }
        }
    }
}

// ── Modifier Group Card ──

@Composable
private fun ModifierGroupCard(
    groupWithModifiers: ModifierGroupWithModifiers,
    onEditGroup: () -> Unit,
    onDeleteGroup: () -> Unit,
    onEditModifier: (com.kiwari.pos.data.model.Modifier) -> Unit,
    onDeleteModifier: (com.kiwari.pos.data.model.Modifier) -> Unit,
    onAddModifier: () -> Unit
) {
    val group = groupWithModifiers.group
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            // Group header
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = group.name,
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        val maxText = if (group.maxSelect != null) "${group.maxSelect}" else "\u221E"
                        Text(
                            text = "Min ${group.minSelect} / Max $maxText",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
                IconButton(onClick = onEditGroup, modifier = Modifier.size(32.dp)) {
                    Icon(
                        imageVector = Icons.Default.Edit,
                        contentDescription = "Edit",
                        modifier = Modifier.size(16.dp)
                    )
                }
                IconButton(onClick = onDeleteGroup, modifier = Modifier.size(32.dp)) {
                    Icon(
                        imageVector = Icons.Default.Delete,
                        contentDescription = "Hapus",
                        tint = MaterialTheme.colorScheme.error,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }

            // Modifiers list
            if (groupWithModifiers.modifiers.isNotEmpty()) {
                Spacer(modifier = Modifier.height(8.dp))
                groupWithModifiers.modifiers.forEach { mod ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = mod.name,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            modifier = Modifier.weight(1f)
                        )
                        val price = mod.price.toBigDecimalOrNull() ?: BigDecimal.ZERO
                        if (price.compareTo(BigDecimal.ZERO) != 0) {
                            Text(
                                text = "+${formatPrice(price)}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                        IconButton(
                            onClick = { onEditModifier(mod) },
                            modifier = Modifier.size(28.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Edit,
                                contentDescription = "Edit",
                                modifier = Modifier.size(14.dp)
                            )
                        }
                        IconButton(
                            onClick = { onDeleteModifier(mod) },
                            modifier = Modifier.size(28.dp)
                        ) {
                            Icon(
                                imageVector = Icons.Default.Delete,
                                contentDescription = "Hapus",
                                tint = MaterialTheme.colorScheme.error,
                                modifier = Modifier.size(14.dp)
                            )
                        }
                    }
                }
            }

            // Add modifier button
            Spacer(modifier = Modifier.height(4.dp))
            TextButton(
                onClick = onAddModifier,
                contentPadding = PaddingValues(horizontal = 8.dp, vertical = 4.dp)
            ) {
                Icon(
                    imageVector = Icons.Default.Add,
                    contentDescription = null,
                    modifier = Modifier.size(16.dp)
                )
                Spacer(modifier = Modifier.width(4.dp))
                Text(
                    text = "Tambah Modifier",
                    style = MaterialTheme.typography.labelMedium
                )
            }
        }
    }
}

// ── Combo Item Row ──

@Composable
private fun ComboItemRow(
    item: ComboItemWithProduct,
    onDelete: () -> Unit
) {
    OutlinedCard(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        border = CardDefaults.outlinedCardBorder()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.productName,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "Qty: ${item.comboItem.quantity}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            IconButton(onClick = onDelete, modifier = Modifier.size(32.dp)) {
                Icon(
                    imageVector = Icons.Default.Delete,
                    contentDescription = "Hapus",
                    tint = MaterialTheme.colorScheme.error,
                    modifier = Modifier.size(16.dp)
                )
            }
        }
    }
}

// ── Bottom Sheet: Variant Group ──

@Composable
private fun VariantGroupSheetContent(
    group: com.kiwari.pos.data.model.VariantGroup?,
    isSaving: Boolean,
    saveError: String?,
    onSave: (name: String, isRequired: Boolean) -> Unit,
    onDelete: (() -> Unit)?
) {
    var name by remember(group?.id) { mutableStateOf(group?.name ?: "") }
    var isRequired by remember(group?.id) { mutableStateOf(group?.isRequired ?: false) }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = if (group != null) "Edit Grup Varian" else "Tambah Grup Varian",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("Nama grup") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving
        )

        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = "Wajib dipilih",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Switch(
                checked = isRequired,
                onCheckedChange = { isRequired = it },
                enabled = !isSaving
            )
        }

        if (saveError != null) {
            Text(
                text = saveError,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
        }

        Button(
            onClick = { onSave(name.trim(), isRequired) },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving && name.isNotBlank(),
            shape = MaterialTheme.shapes.medium
        ) {
            if (isSaving) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary
                )
            } else {
                Text("Simpan")
            }
        }

        if (onDelete != null) {
            TextButton(
                onClick = onDelete,
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving,
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
            ) {
                Text("Hapus Grup Varian")
            }
        }

        Spacer(modifier = Modifier.height(16.dp))
    }
}

// ── Bottom Sheet: Variant ──

@Composable
private fun VariantSheetContent(
    variant: com.kiwari.pos.data.model.Variant?,
    isSaving: Boolean,
    saveError: String?,
    onSave: (name: String, priceAdjustment: String) -> Unit,
    onDelete: (() -> Unit)?
) {
    var name by remember(variant?.id) { mutableStateOf(variant?.name ?: "") }
    var priceAdjustment by remember(variant?.id) { mutableStateOf(variant?.priceAdjustment ?: "0") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = if (variant != null) "Edit Varian" else "Tambah Varian",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("Nama varian") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving
        )

        OutlinedTextField(
            value = priceAdjustment,
            onValueChange = { priceAdjustment = it },
            label = { Text("Selisih harga") },
            singleLine = true,
            prefix = { Text("Rp") },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving,
            supportingText = { Text("Gunakan negatif untuk diskon, mis. -5000") }
        )

        if (saveError != null) {
            Text(
                text = saveError,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
        }

        Button(
            onClick = { onSave(name.trim(), priceAdjustment.trim()) },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving && name.isNotBlank(),
            shape = MaterialTheme.shapes.medium
        ) {
            if (isSaving) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary
                )
            } else {
                Text("Simpan")
            }
        }

        if (onDelete != null) {
            TextButton(
                onClick = onDelete,
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving,
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
            ) {
                Text("Hapus Varian")
            }
        }

        Spacer(modifier = Modifier.height(16.dp))
    }
}

// ── Bottom Sheet: Modifier Group ──

@Composable
private fun ModifierGroupSheetContent(
    group: com.kiwari.pos.data.model.ModifierGroup?,
    isSaving: Boolean,
    saveError: String?,
    onSave: (name: String, minSelect: Int, maxSelect: Int) -> Unit,
    onDelete: (() -> Unit)?
) {
    var name by remember(group?.id) { mutableStateOf(group?.name ?: "") }
    var minSelect by remember(group?.id) { mutableStateOf(group?.minSelect?.toString() ?: "0") }
    var maxSelect by remember(group?.id) { mutableStateOf(group?.maxSelect?.toString() ?: "0") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = if (group != null) "Edit Grup Modifier" else "Tambah Grup Modifier",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("Nama grup") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving
        )

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            OutlinedTextField(
                value = minSelect,
                onValueChange = { minSelect = it },
                label = { Text("Min pilih") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.weight(1f),
                enabled = !isSaving
            )
            OutlinedTextField(
                value = maxSelect,
                onValueChange = { maxSelect = it },
                label = { Text("Max pilih") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.weight(1f),
                enabled = !isSaving
            )
        }

        if (saveError != null) {
            Text(
                text = saveError,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
        }

        Button(
            onClick = {
                onSave(
                    name.trim(),
                    minSelect.toIntOrNull() ?: 0,
                    maxSelect.toIntOrNull() ?: 0
                )
            },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving && name.isNotBlank(),
            shape = MaterialTheme.shapes.medium
        ) {
            if (isSaving) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary
                )
            } else {
                Text("Simpan")
            }
        }

        if (onDelete != null) {
            TextButton(
                onClick = onDelete,
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving,
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
            ) {
                Text("Hapus Grup Modifier")
            }
        }

        Spacer(modifier = Modifier.height(16.dp))
    }
}

// ── Bottom Sheet: Modifier ──

@Composable
private fun ModifierSheetContent(
    modifier: com.kiwari.pos.data.model.Modifier?,
    isSaving: Boolean,
    saveError: String?,
    onSave: (name: String, price: String) -> Unit,
    onDelete: (() -> Unit)?
) {
    var name by remember(modifier?.id) { mutableStateOf(modifier?.name ?: "") }
    var price by remember(modifier?.id) { mutableStateOf(modifier?.price ?: "0") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = if (modifier != null) "Edit Modifier" else "Tambah Modifier",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("Nama modifier") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving
        )

        OutlinedTextField(
            value = price,
            onValueChange = { price = it },
            label = { Text("Harga") },
            singleLine = true,
            prefix = { Text("Rp") },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving
        )

        if (saveError != null) {
            Text(
                text = saveError,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
        }

        Button(
            onClick = { onSave(name.trim(), price.trim()) },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSaving && name.isNotBlank(),
            shape = MaterialTheme.shapes.medium
        ) {
            if (isSaving) {
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp),
                    strokeWidth = 2.dp,
                    color = MaterialTheme.colorScheme.onPrimary
                )
            } else {
                Text("Simpan")
            }
        }

        if (onDelete != null) {
            TextButton(
                onClick = onDelete,
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving,
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error)
            ) {
                Text("Hapus Modifier")
            }
        }

        Spacer(modifier = Modifier.height(16.dp))
    }
}

// ── Bottom Sheet: Combo Item ──

@Composable
private fun ComboItemSheetContent(
    allProducts: List<Product>,
    isSaving: Boolean,
    saveError: String?,
    onAdd: (productId: String, quantity: Int) -> Unit
) {
    var searchQuery by remember { mutableStateOf("") }
    var selectedProduct by remember { mutableStateOf<Product?>(null) }
    var quantity by remember { mutableIntStateOf(1) }

    val filteredProducts = remember(searchQuery, allProducts) {
        if (searchQuery.isBlank()) allProducts
        else allProducts.filter { it.name.contains(searchQuery, ignoreCase = true) }
    }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "Tambah Combo Item",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        if (selectedProduct == null) {
            // Search and pick product
            OutlinedTextField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                label = { Text("Cari produk") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving
            )

            if (filteredProducts.isEmpty()) {
                Text(
                    text = if (searchQuery.isBlank()) "Tidak ada produk tersedia" else "Produk tidak ditemukan",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            } else {
                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(240.dp),
                    verticalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    items(
                        items = filteredProducts,
                        key = { it.id }
                    ) { product ->
                        OutlinedCard(
                            onClick = { selectedProduct = product },
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.small,
                            border = CardDefaults.outlinedCardBorder()
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(12.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = product.name,
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    modifier = Modifier.weight(1f)
                                )
                                val price = product.basePrice.toBigDecimalOrNull() ?: BigDecimal.ZERO
                                Text(
                                    text = formatPrice(price),
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
            }
        } else {
            // Selected product - show name and quantity picker
            val product = selectedProduct ?: return@Column
            OutlinedCard(
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.medium,
                border = CardDefaults.outlinedCardBorder()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(12.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = product.name,
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onSurface,
                        modifier = Modifier.weight(1f)
                    )
                    TextButton(onClick = { selectedProduct = null }) {
                        Text("Ganti")
                    }
                }
            }

            // Quantity
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Text(
                    text = "Jumlah:",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                IconButton(
                    onClick = { if (quantity > 1) quantity-- },
                    enabled = quantity > 1 && !isSaving,
                    modifier = Modifier.size(36.dp)
                ) {
                    Text(
                        text = "\u2212",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
                Text(
                    text = "$quantity",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                IconButton(
                    onClick = { quantity++ },
                    enabled = !isSaving,
                    modifier = Modifier.size(36.dp)
                ) {
                    Text(
                        text = "+",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
            }

            if (saveError != null) {
                Text(
                    text = saveError,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error
                )
            }

            Button(
                onClick = { onAdd(product.id, quantity) },
                modifier = Modifier.fillMaxWidth(),
                enabled = !isSaving,
                shape = MaterialTheme.shapes.medium
            ) {
                if (isSaving) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                } else {
                    Text("Tambah")
                }
            }
        }

        Spacer(modifier = Modifier.height(16.dp))
    }
}
