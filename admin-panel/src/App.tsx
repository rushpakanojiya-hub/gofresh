import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import ProtectedRoute from './components/ProtectedRoute'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Products from './pages/Products'
import Customers from './pages/Customers'
import InventoryOverview from './pages/InventoryOverview'
import StaffRoles from './pages/StaffRoles'
import Settings from './pages/Settings'
import Categories from './pages/Categories'
import Orders from './pages/Orders'
import Coupons from './pages/Coupons'
import DeliveryPartners from './pages/DeliveryPartners'
import Warehouses from './pages/Warehouses'
import WarehouseStaff from './pages/WarehouseStaff'
import StockTransfers from './pages/StockTransfers'
import Returns from './pages/Returns'
import Analytics from './pages/Analytics'
import PickerPerformance from './pages/PickerPerformance'
import AuditLogs from './pages/AuditLogs'
import Notifications from './pages/Notifications'
import Offers from './pages/Offers'
import Banners from './pages/Banners'
import DeliveryZones from './pages/DeliveryZones'
import SupportTickets from './pages/SupportTickets'
import ControlTower from './pages/ControlTower'
import UnassignedOrders from './pages/UnassignedOrders'
import DeliveryManagement from './pages/DeliveryManagement'
import OperationsAnalytics from './pages/OperationsAnalytics'
import Payments from './pages/Payments'
import Invoices from './pages/Invoices'
import WalletCredit from './pages/WalletCredit'
import Suppliers from './pages/Suppliers'
import PurchaseOrders from './pages/PurchaseOrders'
import Replenishment from './pages/Replenishment'

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
          <Route
            path="/products"
            element={
              <ProtectedRoute>
                <Products />
              </ProtectedRoute>
            }
          />
          <Route
            path="/categories"
            element={
              <ProtectedRoute>
                <Categories />
              </ProtectedRoute>
            }
          />
          <Route
            path="/orders"
            element={
              <ProtectedRoute>
                <Orders />
              </ProtectedRoute>
            }
          />
          <Route
            path="/coupons"
            element={
              <ProtectedRoute>
                <Coupons />
              </ProtectedRoute>
            }
          />
          <Route
            path="/delivery-partners"
            element={
              <ProtectedRoute>
                <DeliveryPartners />
              </ProtectedRoute>
            }
          />
          <Route
            path="/warehouses"
            element={
              <ProtectedRoute>
                <Warehouses />
              </ProtectedRoute>
            }
          />
          <Route
            path="/warehouse-staff"
            element={
              <ProtectedRoute>
                <WarehouseStaff />
              </ProtectedRoute>
            }
          />
          <Route
            path="/stock-transfers"
            element={
              <ProtectedRoute>
                <StockTransfers />
              </ProtectedRoute>
            }
          />
          <Route
            path="/returns"
            element={
              <ProtectedRoute>
                <Returns />
              </ProtectedRoute>
            }
          />
          <Route
            path="/analytics"
            element={
              <ProtectedRoute>
                <Analytics />
              </ProtectedRoute>
            }
          />
          <Route
            path="/wallet-credit"
            element={
              <ProtectedRoute>
                <WalletCredit />
              </ProtectedRoute>
            }
          />
          <Route
            path="/customers"
            element={
              <ProtectedRoute>
                <Customers />
              </ProtectedRoute>
            }
          />
          <Route
            path="/inventory"
            element={
              <ProtectedRoute>
                <InventoryOverview />
              </ProtectedRoute>
            }
          />
          <Route
            path="/staff-roles"
            element={
              <ProtectedRoute>
                <StaffRoles />
              </ProtectedRoute>
            }
          />
          <Route
            path="/settings"
            element={
              <ProtectedRoute>
                <Settings />
              </ProtectedRoute>
            }
          />
          <Route
            path="/audit-logs"
            element={
              <ProtectedRoute>
                <AuditLogs />
              </ProtectedRoute>
            }
          />

          <Route
            path="/picker-performance"
            element={
              <ProtectedRoute>
                <PickerPerformance />
              </ProtectedRoute>
            }
          />
          <Route
            path="/notifications"
            element={
              <ProtectedRoute>
                <Notifications />
              </ProtectedRoute>
            }
          />
          <Route
            path="/offers"
            element={
              <ProtectedRoute>
                <Offers />
              </ProtectedRoute>
            }
          />
          <Route
            path="/banners"
            element={
              <ProtectedRoute>
                <Banners />
              </ProtectedRoute>
            }
          />
          <Route
            path="/delivery-zones"
            element={
              <ProtectedRoute>
                <DeliveryZones />
              </ProtectedRoute>
            }
          />
          <Route
            path="/support"
            element={
              <ProtectedRoute>
                <SupportTickets />
              </ProtectedRoute>
            }
          />
          <Route
            path="/control-tower"
            element={
              <ProtectedRoute>
                <ControlTower />
              </ProtectedRoute>
            }
          />
          <Route
            path="/unassigned-orders"
            element={
              <ProtectedRoute>
                <UnassignedOrders />
              </ProtectedRoute>
            }
          />
          <Route
            path="/delivery-management"
            element={
              <ProtectedRoute>
                <DeliveryManagement />
              </ProtectedRoute>
            }
          />
          <Route
            path="/operations-analytics"
            element={
              <ProtectedRoute>
                <OperationsAnalytics />
              </ProtectedRoute>
            }
          />
          <Route
            path="/payments"
            element={
              <ProtectedRoute>
                <Payments />
              </ProtectedRoute>
            }
          />
          <Route
            path="/invoices"
            element={
              <ProtectedRoute>
                <Invoices />
              </ProtectedRoute>
            }
          />
          <Route
            path="/suppliers"
            element={
              <ProtectedRoute>
                <Suppliers />
              </ProtectedRoute>
            }
          />
          <Route
            path="/purchase-orders"
            element={
              <ProtectedRoute>
                <PurchaseOrders />
              </ProtectedRoute>
            }
          />
          <Route
            path="/replenishment"
            element={
              <ProtectedRoute>
                <Replenishment />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App

