import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { GuestOnly, RequireAuth, RequireStreamer } from './components/RouteGuards'
import { LoginPage, RegisterPage } from './pages/AuthPages'
import { HomePage } from './pages/HomePage'
import { QueuePage } from './pages/QueuePage'
import { SentPage } from './pages/SentPage'
import { SettingsPage } from './pages/SettingsPage'
import { StreamerPage } from './pages/StreamerPage'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="s/:username" element={<StreamerPage />} />

        <Route
          path="login"
          element={
            <GuestOnly>
              <LoginPage />
            </GuestOnly>
          }
        />
        <Route
          path="register"
          element={
            <GuestOnly>
              <RegisterPage />
            </GuestOnly>
          }
        />

        <Route
          path="sent"
          element={
            <RequireAuth>
              <SentPage />
            </RequireAuth>
          }
        />
        <Route
          path="settings"
          element={
            <RequireAuth>
              <SettingsPage />
            </RequireAuth>
          }
        />
        <Route
          path="queue"
          element={
            <RequireStreamer>
              <QueuePage />
            </RequireStreamer>
          }
        />

        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
