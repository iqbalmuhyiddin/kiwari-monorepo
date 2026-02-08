package com.kiwari.pos.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.ui.cart.CartScreen
import com.kiwari.pos.ui.catering.CateringScreen
import com.kiwari.pos.ui.login.LoginScreen
import com.kiwari.pos.ui.menu.CustomizationScreen
import com.kiwari.pos.ui.menu.MenuScreen
import com.kiwari.pos.ui.orders.OrderDetailScreen
import com.kiwari.pos.ui.payment.PaymentScreen
import com.kiwari.pos.ui.settings.PrinterSettingsScreen

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Menu : Screen("menu")
    object Cart : Screen("cart?editOrderId={editOrderId}") {
        fun createRoute(editOrderId: String? = null): String {
            return if (editOrderId != null) "cart?editOrderId=$editOrderId" else "cart"
        }
    }
    object Payment : Screen("payment")
    object Catering : Screen("catering")
    object Customization : Screen("customization/{productId}") {
        fun createRoute(productId: String) = "customization/$productId"
    }
    object Settings : Screen("settings")
    object OrderDetail : Screen("orderDetail/{orderId}") {
        fun createRoute(orderId: String) = "orderDetail/$orderId"
    }
}

@Composable
fun NavGraph(
    navController: NavHostController,
    tokenRepository: TokenRepository,
    modifier: Modifier = Modifier
) {
    val isLoggedIn by tokenRepository.isLoggedIn.collectAsState(initial = false)

    // Handle logout navigation - navigate back to Login when isLoggedIn becomes false
    LaunchedEffect(isLoggedIn) {
        if (!isLoggedIn) {
            navController.navigate(Screen.Login.route) {
                popUpTo(0) { inclusive = true }
            }
        }
    }

    NavHost(
        navController = navController,
        startDestination = if (isLoggedIn) Screen.Menu.route else Screen.Login.route,
        modifier = modifier
    ) {
        composable(Screen.Login.route) {
            LoginScreen(
                onLoginSuccess = {
                    // Navigate to Menu and clear back stack
                    navController.navigate(Screen.Menu.route) {
                        popUpTo(Screen.Login.route) { inclusive = true }
                    }
                }
            )
        }

        composable(Screen.Menu.route) {
            MenuScreen(
                onNavigateToCart = {
                    navController.navigate(Screen.Cart.createRoute())
                },
                onNavigateToCustomization = { productId ->
                    navController.navigate(Screen.Customization.createRoute(productId))
                },
                onNavigateToSettings = {
                    navController.navigate(Screen.Settings.route)
                }
            )
        }

        composable(
            route = Screen.Cart.route,
            arguments = listOf(navArgument("editOrderId") {
                type = NavType.StringType
                nullable = true
                defaultValue = null
            })
        ) {
            CartScreen(
                onNavigateBack = {
                    navController.popBackStack()
                },
                onNavigateToPayment = {
                    navController.navigate(Screen.Payment.route)
                },
                onNavigateToCatering = {
                    navController.navigate(Screen.Catering.route)
                },
                onNavigateToOrderDetail = { orderId ->
                    navController.navigate(Screen.OrderDetail.createRoute(orderId)) {
                        popUpTo(Screen.Menu.route)
                    }
                }
            )
        }

        composable(Screen.Payment.route) {
            PaymentScreen(
                onNavigateBack = {
                    navController.popBackStack()
                },
                onNavigateToMenu = {
                    navController.navigate(Screen.Menu.route) {
                        popUpTo(Screen.Menu.route) { inclusive = true }
                    }
                }
            )
        }

        composable(Screen.Catering.route) {
            CateringScreen(
                onNavigateBack = {
                    navController.popBackStack()
                },
                onNavigateToMenu = {
                    navController.navigate(Screen.Menu.route) {
                        popUpTo(Screen.Menu.route) { inclusive = true }
                    }
                }
            )
        }

        composable(
            route = Screen.Customization.route,
            arguments = listOf(navArgument("productId") { type = NavType.StringType })
        ) {
            CustomizationScreen(
                onNavigateBack = {
                    navController.popBackStack()
                }
            )
        }

        composable(Screen.Settings.route) {
            PrinterSettingsScreen(
                onNavigateBack = {
                    navController.popBackStack()
                }
            )
        }

        composable(
            route = Screen.OrderDetail.route,
            arguments = listOf(navArgument("orderId") { type = NavType.StringType })
        ) {
            OrderDetailScreen(
                onBack = {
                    navController.popBackStack()
                },
                onPay = { orderId ->
                    // TODO: navigate to payment for existing order
                },
                onEdit = { orderId ->
                    navController.navigate(Screen.Cart.createRoute(editOrderId = orderId)) {
                        popUpTo(Screen.Menu.route)
                    }
                }
            )
        }
    }
}
