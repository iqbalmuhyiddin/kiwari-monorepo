package com.kiwari.pos.ui.navigation

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.ui.login.LoginScreen
import com.kiwari.pos.ui.menu.MenuScreen

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Menu : Screen("menu")
    object Cart : Screen("cart")
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
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Coming soon")
            }
        }

        composable(
            route = Screen.Customization.route,
            arguments = listOf(navArgument("productId") { type = NavType.StringType })
        ) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Coming soon")
            }
        }
    }
}
