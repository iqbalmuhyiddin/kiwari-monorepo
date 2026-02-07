package com.kiwari.pos.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.ui.login.LoginScreen
import com.kiwari.pos.ui.menu.MenuScreen

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Menu : Screen("menu")
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
            MenuScreen()
        }
    }
}
