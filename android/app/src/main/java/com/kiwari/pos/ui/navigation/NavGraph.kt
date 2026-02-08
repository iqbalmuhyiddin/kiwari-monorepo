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
import com.kiwari.pos.ui.login.LoginScreen
import com.kiwari.pos.ui.menu.CustomizationScreen
import com.kiwari.pos.ui.menu.MenuScreen
import com.kiwari.pos.ui.payment.PaymentScreen

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Menu : Screen("menu")
    object Cart : Screen("cart")
    object Payment : Screen("payment")
    object Customization : Screen("customization/{productId}") {
        fun createRoute(productId: String) = "customization/$productId"
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
                    navController.navigate(Screen.Cart.route)
                },
                onNavigateToCustomization = { productId ->
                    navController.navigate(Screen.Customization.createRoute(productId))
                }
            )
        }

        composable(Screen.Cart.route) {
            CartScreen(
                onNavigateBack = {
                    navController.popBackStack()
                },
                onNavigateToPayment = {
                    navController.navigate(Screen.Payment.route)
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
    }
}
