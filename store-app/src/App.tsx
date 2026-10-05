import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import ProtectedRoute from './components/ProtectedRoute'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Orders from './pages/Orders'
import Picking from './pages/Picking'
import Packing from './pages/Packing'
import Substitution from './pages/Substitution'
import Returns from './pages/Returns'
import Inventory from './pages/Inventory'
import Exceptions from './pages/Exceptions'
import Handover from './pages/Handover'
import Performance from './pages/Performance'
import Staff from './pages/Staff'
import StoreQR from './pages/StoreQR'

function Protected({ children }: { children: React.ReactNode }) {
  return (
    <ProtectedRoute>
      <Layout>{children}</Layout>
    </ProtectedRoute>
  )
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/dashboard" element={<Protected><Dashboard /></Protected>} />
          <Route path="/orders" element={<Protected><Orders /></Protected>} />
          <Route path="/picking/:orderId" element={<Protected><Picking /></Protected>} />
          <Route path="/packing/:orderId" element={<Protected><Packing /></Protected>} />
          <Route path="/substitutions" element={<Protected><Substitution /></Protected>} />
          <Route path="/returns" element={<Protected><Returns /></Protected>} />
          <Route path="/inventory" element={<Protected><Inventory /></Protected>} />
          <Route path="/exceptions" element={<Protected><Exceptions /></Protected>} />
          <Route path="/handover" element={<Protected><Handover /></Protected>} />
          <Route path="/performance" element={<Protected><Performance /></Protected>} />
          <Route path="/staff" element={<Protected><Staff /></Protected>} />
          <Route path="/checkin-qr" element={<Protected><StoreQR /></Protected>} />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}

export default App


