package com.kiwari.pos.ui.navigation

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.DrawerState
import androidx.compose.material3.ModalNavigationDrawer
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.kiwari.pos.data.model.UserRole
import com.kiwari.pos.data.repository.TokenRepository
import com.kiwari.pos.ui.cart.CartScreen
import com.kiwari.pos.ui.catering.CateringScreen
import com.kiwari.pos.ui.login.LoginScreen
import com.kiwari.pos.ui.menu.CustomizationScreen
import com.kiwari.pos.ui.menu.MenuScreen
import com.kiwari.pos.ui.orders.OrderDetailScreen
import com.kiwari.pos.ui.orders.OrderListScreen
import com.kiwari.pos.ui.payment.PaymentScreen
import com.kiwari.pos.ui.customers.CustomerDetailScreen
import com.kiwari.pos.ui.customers.CustomerListScreen
import com.kiwari.pos.ui.menuadmin.CategoryListScreen
import com.kiwari.pos.ui.reports.ReportsScreen
import com.kiwari.pos.ui.settings.PrinterSettingsScreen
import com.kiwari.pos.util.DrawerFeature
import kotlinx.coroutines.launch

sealed class Screen(val route: String) {
    object Login : Screen("login")
    object Menu : Screen("menu")
    object Cart : Screen("cart?editOrderId={editOrderId}") {
        fun createRoute(editOrderId: String? = null): String {
            return if (editOrderId != null) "cart?editOrderId=$editOrderId" else "cart"
        }
    }
    object Payment : Screen("payment?orderId={orderId}") {
        fun createRoute(orderId: String? = null): String {
            return if (orderId != null) "payment?orderId=$orderId" else "payment"
        }
    }
    object Catering : Screen("catering")
    object Customization : Screen("customization/{productId}") {
        fun createRoute(productId: String) = "customization/$productId"
    }
    object OrderList : Screen("order-list")
    object Settings : Screen("settings")
    object OrderDetail : Screen("orderDetail/{orderId}") {
        fun createRoute(orderId: String) = "orderDetail/$orderId"
    }
    object Reports : Screen("reports")
    object MenuAdmin : Screen("menu-admin")
    object CustomerList : Screen("customers")
    object CustomerDetail : Screen("customer/{customerId}") {
        fun createRoute(customerId: String) = "customer/$customerId"
    }
    object StaffList : Screen("staff")
}

@Composable
fun NavGraph(
    navController: NavHostController,
    tokenRepository: TokenRepository,
    drawerState: DrawerState,
    modifier: Modifier = Modifier
) {
    val isLoggedIn by tokenRepository.isLoggedIn.collectAsState(initial = false)
    val scope = rememberCoroutineScope()

    val userName = tokenRepository.getUserName() ?: "User"
    val userRoleStr = tokenRepository.getUserRole()
    val userRole = try {
        UserRole.valueOf(userRoleStr ?: "CASHIER")
    } catch (e: Exception) {
        UserRole.CASHIER
    }

    // Handle logout navigation - navigate back to Login when isLoggedIn becomes false
    LaunchedEffect(isLoggedIn) {
        if (!isLoggedIn) {
            drawerState.close()
            navController.navigate(Screen.Login.route) {
                popUpTo(0) { inclusive = true }
            }
        }
    }

    ModalNavigationDrawer(
        drawerState = drawerState,
        gesturesEnabled = drawerState.isOpen,
        drawerContent = {
            AppDrawerContent(
                userName = userName,
                userRole = userRole,
                outletName = "",
                onItemClick = { feature ->
                    scope.launch { drawerState.close() }
                    when (feature) {
                        DrawerFeature.PESANAN -> navController.navigate(Screen.OrderList.route) { launchSingleTop = true }
                        DrawerFeature.LAPORAN -> navController.navigate(Screen.Reports.route) { launchSingleTop = true }
                        DrawerFeature.MENU_ADMIN -> navController.navigate(Screen.MenuAdmin.route) { launchSingleTop = true }
                        DrawerFeature.PELANGGAN -> navController.navigate(Screen.CustomerList.route) { launchSingleTop = true }
                        DrawerFeature.PENGGUNA -> navController.navigate(Screen.StaffList.route) { launchSingleTop = true }
                        DrawerFeature.PRINTER -> navController.navigate(Screen.Settings.route) { launchSingleTop = true }
                    }
                },
                onLogout = {
                    scope.launch { drawerState.close() }
                    tokenRepository.clearTokens()
                }
            )
        }
    ) {
        NavHost(
            navController = navController,
            startDestination = if (isLoggedIn) Screen.Menu.route else Screen.Login.route,
            modifier = modifier
        ) {
            composable(Screen.Login.route) {
                LoginScreen(
                    onLoginSuccess = {
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
                    onMenuClick = {
                        scope.launch { drawerState.open() }
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
                        navController.navigate(Screen.Payment.createRoute())
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

            composable(
                route = Screen.Payment.route,
                arguments = listOf(navArgument("orderId") {
                    type = NavType.StringType
                    nullable = true
                    defaultValue = null
                })
            ) {
                PaymentScreen(
                    onNavigateBack = {
                        navController.popBackStack()
                    },
                    onNavigateToMenu = {
                        navController.navigate(Screen.Menu.route) {
                            popUpTo(Screen.Menu.route) { inclusive = true }
                        }
                    },
                    onNavigateToOrderDetail = { orderId ->
                        navController.navigate(Screen.OrderDetail.createRoute(orderId)) {
                            popUpTo(Screen.Menu.route)
                        }
                    }
                )
            }

            composable(Screen.Catering.route) {
                CateringScreen(
                    onNavigateBack = {
                        navController.popBackStack()
                    },
                    onNavigateToOrderDetail = { orderId ->
                        navController.navigate(Screen.OrderDetail.createRoute(orderId)) {
                            popUpTo(Screen.Menu.route)
                        }
                    }
                )
            }

            composable(Screen.OrderList.route) {
                OrderListScreen(
                    onOrderClick = { orderId ->
                        navController.navigate(Screen.OrderDetail.createRoute(orderId)) {
                            launchSingleTop = true
                        }
                    },
                    onNavigateBack = {
                        navController.popBackStack()
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
                        navController.navigate(Screen.Payment.createRoute(orderId = orderId))
                    },
                    onEdit = { orderId ->
                        navController.navigate(Screen.Cart.createRoute(editOrderId = orderId)) {
                            popUpTo(Screen.Menu.route)
                        }
                    }
                )
            }

            // Placeholder screens for new drawer destinations
            composable(Screen.Reports.route) {
                ReportsScreen(onNavigateBack = { navController.popBackStack() })
            }

            composable(Screen.MenuAdmin.route) {
                CategoryListScreen(
                    onCategoryClick = { categoryId, categoryName ->
                        // ProductList will be wired in Task 9
                    },
                    onNavigateBack = { navController.popBackStack() }
                )
            }

            composable(Screen.CustomerList.route) {
                CustomerListScreen(
                    onCustomerClick = { customerId ->
                        navController.navigate(Screen.CustomerDetail.createRoute(customerId)) {
                            launchSingleTop = true
                        }
                    },
                    onNavigateBack = { navController.popBackStack() }
                )
            }

            composable(
                route = Screen.CustomerDetail.route,
                arguments = listOf(navArgument("customerId") { type = NavType.StringType })
            ) {
                CustomerDetailScreen(
                    onNavigateBack = { navController.popBackStack() },
                    onOrderClick = { orderId ->
                        navController.navigate(Screen.OrderDetail.createRoute(orderId)) {
                            launchSingleTop = true
                        }
                    }
                )
            }

            composable(Screen.StaffList.route) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    Text("Coming soon")
                }
            }
        }
    }
}
