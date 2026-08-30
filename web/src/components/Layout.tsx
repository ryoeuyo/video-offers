import { Link, NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import styles from './Layout.module.css'

export function Layout() {
  const { user, logout, loading } = useAuth()

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>
          <span className={styles.logoMark}>▶</span>
          OfferBox
        </Link>

        <nav className={styles.nav}>
          <NavLink to="/" end className={({ isActive }) => (isActive ? styles.active : '')}>
            Стримеры
          </NavLink>
          {user && (
            <NavLink to="/sent" className={({ isActive }) => (isActive ? styles.active : '')}>
              Мои предложения
            </NavLink>
          )}
          {user && (
            <NavLink to="/settings" className={({ isActive }) => (isActive ? styles.active : '')}>
              Настройки
            </NavLink>
          )}
          {user?.role === 'streamer' && (
            <NavLink to="/queue" className={({ isActive }) => (isActive ? styles.active : '')}>
              Очередь
            </NavLink>
          )}
        </nav>

        <div className={styles.actions}>
          {!loading && user ? (
            <>
              <Link to={`/s/${user.username}`} className={styles.userChip}>
                @{user.username}
              </Link>
              <button type="button" className={styles.ghostBtn} onClick={() => logout()}>
                Выйти
              </button>
            </>
          ) : (
            !loading && (
              <>
                <Link to="/login" className={styles.ghostBtn}>
                  Войти
                </Link>
                <Link to="/register" className={styles.primaryBtn}>
                  Регистрация
                </Link>
              </>
            )
          )}
        </div>
      </header>

      <main className={styles.main}>
        <Outlet />
      </main>

      <footer className={styles.footer}>
        Предложка видео для стримеров — ссылки на YouTube, Twitch и др.
      </footer>
    </div>
  )
}
