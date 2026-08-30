import { type FormEvent, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import styles from './AuthForm.module.css'

interface AuthFormProps {
  mode: 'login' | 'register'
}

export function AuthForm({ mode }: AuthFormProps) {
  const { login, register } = useAuth()
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      if (mode === 'login') {
        await login(email, password)
      } else {
        await register(email, username, password)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.card}>
        <h1>{mode === 'login' ? 'Вход' : 'Регистрация'}</h1>
        <p className={styles.sub}>
          {mode === 'login'
            ? 'Войдите, чтобы предлагать видео стримерам'
            : 'Создайте аккаунт — можно стать стримером позже'}
        </p>

        <form onSubmit={handleSubmit} className={styles.form}>
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
            />
          </label>

          {mode === 'register' && (
            <label>
              Username
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                minLength={3}
                maxLength={32}
                pattern="[a-zA-Z0-9_]+"
                title="3–32 символа: буквы, цифры, _"
                autoComplete="username"
              />
            </label>
          )}

          <label>
            Пароль
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            />
          </label>

          {error && <div className={styles.error}>{error}</div>}

          <button type="submit" className={styles.submit} disabled={submitting}>
            {submitting ? '...' : mode === 'login' ? 'Войти' : 'Создать аккаунт'}
          </button>
        </form>

        <p className={styles.switch}>
          {mode === 'login' ? (
            <>
              Нет аккаунта? <Link to="/register">Зарегистрироваться</Link>
            </>
          ) : (
            <>
              Уже есть аккаунт? <Link to="/login">Войти</Link>
            </>
          )}
        </p>
      </div>
    </div>
  )
}
