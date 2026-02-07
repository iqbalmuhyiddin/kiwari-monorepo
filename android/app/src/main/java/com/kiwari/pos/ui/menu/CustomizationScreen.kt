package com.kiwari.pos.ui.menu

import androidx.compose.foundation.background
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.kiwari.pos.ui.theme.PrimaryGreen
import com.kiwari.pos.ui.theme.White
import com.kiwari.pos.util.formatPrice
import java.math.BigDecimal

@Composable
fun CustomizationScreen(
    viewModel: CustomizationViewModel = hiltViewModel(),
    onNavigateBack: () -> Unit = {}
) {
    val uiState by viewModel.uiState.collectAsState()

    // Navigate back after adding to cart
    LaunchedEffect(uiState.addedToCart) {
        if (uiState.addedToCart) {
            viewModel.onAddedToCartHandled()
            onNavigateBack()
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        // Top bar
        CustomizationTopBar(
            productName = uiState.product?.name ?: "",
            onBack = onNavigateBack
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
                        Spacer(modifier = Modifier.height(16.dp))
                        TextButton(onClick = viewModel::retry) {
                            Text("Coba lagi")
                        }
                    }
                }
            }

            else -> {
                // Scrollable content
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .verticalScroll(rememberScrollState())
                        .padding(horizontal = 16.dp)
                ) {
                    Spacer(modifier = Modifier.height(12.dp))

                    // Product name + base price
                    uiState.product?.let { product ->
                        Text(
                            text = product.name,
                            style = MaterialTheme.typography.headlineSmall,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text = formatPrice(BigDecimal(product.basePrice)),
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        product.description?.takeIf { it.isNotBlank() }?.let { desc ->
                            Spacer(modifier = Modifier.height(4.dp))
                            Text(
                                text = desc,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }

                    // Variant groups
                    if (uiState.variantGroups.isNotEmpty()) {
                        Spacer(modifier = Modifier.height(16.dp))
                        HorizontalDivider()

                        for (vgWithVariants in uiState.variantGroups) {
                            Spacer(modifier = Modifier.height(12.dp))
                            VariantGroupSection(
                                groupName = vgWithVariants.group.name,
                                isRequired = vgWithVariants.group.isRequired,
                                variants = vgWithVariants.variants,
                                selectedVariantId = uiState.selectedVariants[vgWithVariants.group.id],
                                onVariantSelected = { variantId ->
                                    viewModel.onVariantSelected(vgWithVariants.group.id, variantId)
                                }
                            )
                        }
                    }

                    // Modifier groups
                    if (uiState.modifierGroups.isNotEmpty()) {
                        Spacer(modifier = Modifier.height(16.dp))
                        HorizontalDivider()

                        for (mgWithModifiers in uiState.modifierGroups) {
                            Spacer(modifier = Modifier.height(12.dp))
                            ModifierGroupSection(
                                group = mgWithModifiers.group,
                                modifiers = mgWithModifiers.modifiers,
                                selectedModifierIds = uiState.selectedModifiers[mgWithModifiers.group.id]
                                    ?: emptySet(),
                                onModifierToggled = { modifierId ->
                                    viewModel.onModifierToggled(mgWithModifiers.group.id, modifierId)
                                }
                            )
                        }
                    }

                    // Quantity selector
                    Spacer(modifier = Modifier.height(16.dp))
                    HorizontalDivider()
                    Spacer(modifier = Modifier.height(12.dp))
                    QuantitySelector(
                        quantity = uiState.quantity,
                        onQuantityChanged = viewModel::onQuantityChanged
                    )

                    // Notes
                    Spacer(modifier = Modifier.height(16.dp))
                    HorizontalDivider()
                    Spacer(modifier = Modifier.height(12.dp))
                    Text(
                        text = "Catatan",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = uiState.notes,
                        onValueChange = viewModel::onNotesChanged,
                        modifier = Modifier.fillMaxWidth(),
                        placeholder = { Text("Contoh: tidak pedas, tanpa bawang...") },
                        maxLines = 3,
                        singleLine = false
                    )

                    // Spacing before bottom button
                    Spacer(modifier = Modifier.height(80.dp))
                }

                // Add to cart button — fixed at bottom
                AddToCartButton(
                    total = uiState.calculatedTotal,
                    enabled = uiState.canAddToCart,
                    onClick = viewModel::addToCart
                )
            }
        }
    }
}

@Composable
private fun CustomizationTopBar(
    productName: String,
    onBack: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .padding(horizontal = 4.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        IconButton(onClick = onBack) {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "Kembali",
                tint = MaterialTheme.colorScheme.onSurface
            )
        }
        Text(
            text = "Kustomisasi",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

@Composable
private fun VariantGroupSection(
    groupName: String,
    isRequired: Boolean,
    variants: List<com.kiwari.pos.data.model.Variant>,
    selectedVariantId: String?,
    onVariantSelected: (String) -> Unit
) {
    Column {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = groupName,
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold,
                color = MaterialTheme.colorScheme.onSurface
            )
            if (isRequired) {
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = "Wajib",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error,
                    fontWeight = FontWeight.Medium
                )
            }
        }
        Spacer(modifier = Modifier.height(4.dp))

        for (variant in variants) {
            val priceAdj = BigDecimal(variant.priceAdjustment)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { onVariantSelected(variant.id) }
                    .padding(vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                RadioButton(
                    selected = variant.id == selectedVariantId,
                    onClick = { onVariantSelected(variant.id) },
                    colors = RadioButtonDefaults.colors(
                        selectedColor = PrimaryGreen
                    )
                )
                Text(
                    text = variant.name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f)
                )
                if (priceAdj.compareTo(BigDecimal.ZERO) != 0) {
                    val prefix = if (priceAdj > BigDecimal.ZERO) "+" else ""
                    Text(
                        text = "$prefix${formatPrice(priceAdj)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Composable
private fun ModifierGroupSection(
    group: com.kiwari.pos.data.model.ModifierGroup,
    modifiers: List<com.kiwari.pos.data.model.Modifier>,
    selectedModifierIds: Set<String>,
    onModifierToggled: (String) -> Unit
) {
    Column {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = group.name,
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.SemiBold,
                color = MaterialTheme.colorScheme.onSurface
            )
            Spacer(modifier = Modifier.width(6.dp))
            val hint = buildConstraintHint(group.minSelect, group.maxSelect)
            if (hint.isNotEmpty()) {
                Text(
                    text = hint,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        Spacer(modifier = Modifier.height(4.dp))

        for (modifier in modifiers) {
            val price = BigDecimal(modifier.price)
            val isSelected = modifier.id in selectedModifierIds
            val atMax = group.maxSelect != null && selectedModifierIds.size >= group.maxSelect!! && !isSelected

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(enabled = !atMax || isSelected) {
                        onModifierToggled(modifier.id)
                    }
                    .padding(vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Checkbox(
                    checked = isSelected,
                    onCheckedChange = {
                        if (!atMax || isSelected) {
                            onModifierToggled(modifier.id)
                        }
                    },
                    enabled = !atMax || isSelected,
                    colors = CheckboxDefaults.colors(
                        checkedColor = PrimaryGreen
                    )
                )
                Text(
                    text = modifier.name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = if (atMax && !isSelected)
                        MaterialTheme.colorScheme.onSurfaceVariant
                    else
                        MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f)
                )
                if (price.compareTo(BigDecimal.ZERO) != 0) {
                    Text(
                        text = "+${formatPrice(price)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

private fun buildConstraintHint(minSelect: Int, maxSelect: Int?): String {
    return when {
        minSelect > 0 && maxSelect != null && minSelect == maxSelect -> "Pilih $minSelect"
        minSelect > 0 && maxSelect != null -> "Pilih $minSelect-$maxSelect"
        minSelect > 0 -> "Min. $minSelect"
        maxSelect != null -> "Maks. $maxSelect"
        else -> ""
    }
}

@Composable
private fun QuantitySelector(
    quantity: Int,
    onQuantityChanged: (Int) -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = "Jumlah",
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.SemiBold,
            color = MaterialTheme.colorScheme.onSurface
        )

        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .background(
                        color = if (quantity > 1) MaterialTheme.colorScheme.surfaceVariant
                        else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                        shape = CircleShape
                    )
                    .then(
                        if (quantity > 1) Modifier.clickable { onQuantityChanged(quantity - 1) }
                        else Modifier
                    ),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "-",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = if (quantity > 1) MaterialTheme.colorScheme.onSurface
                    else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f)
                )
            }

            Text(
                text = "$quantity",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                textAlign = TextAlign.Center,
                modifier = Modifier.width(32.dp)
            )

            Box(
                modifier = Modifier
                    .size(36.dp)
                    .background(
                        color = MaterialTheme.colorScheme.surfaceVariant,
                        shape = CircleShape
                    )
                    .clickable { onQuantityChanged(quantity + 1) },
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = "+",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}

@Composable
private fun AddToCartButton(
    total: BigDecimal,
    enabled: Boolean,
    onClick: () -> Unit
) {
    Button(
        onClick = onClick,
        enabled = enabled,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp)
            .height(52.dp),
        shape = RoundedCornerShape(12.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = PrimaryGreen,
            contentColor = White,
            disabledContainerColor = PrimaryGreen.copy(alpha = 0.4f),
            disabledContentColor = White.copy(alpha = 0.6f)
        )
    ) {
        Text(
            text = "TAMBAH KE KERANJANG  -  ${formatPrice(total)}",
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.Bold
        )
    }
}
