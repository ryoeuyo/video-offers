import { type FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../context/AuthContext'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const { user, refreshUser } = useAuth()
  const [displayName, setDisplayName] = useState('')
  const [acceptingOffers, setAcceptingOffers] = useState(true)
  const [loadingSettings, setLoadingSettings] = useState(false)
  const [saving, setSaving] = useState(false)
  const [becomingStreamer, setBecomingStreamer] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (user) {
      setDisplayName(user.display_name)
    }
  }, [user])

  useEffect(() => {
    if (user?.role !== 'streamer') return
    setLoadingSettings(true)
    api
      .getSettings()
      .then((s) => setAcceptingOffers(s.accepting_offers))
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка'))
      .finally(() => setLoadingSettings(false))
  }, [user?.role])

  async function handleProfile(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    setMessage('')
    try {
      await api.updateMe({ display_name: displayName.trim() || undefined })
      await refreshUser()
      setMessage('Профиль сохранён')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setSaving(false)
    }
  }

  async function becomeStreamer() {
    setBecomingStreamer(true)
    setError('')
    setMessage('')
    try {
      await api.updateMe({ role: 'streamer' })
      await refreshUser()
      setMessage('Вы стали стримером! Настройте приём предложений ниже.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setBecomingStreamer(false)
    }
  }

  async function handleSettings(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    setMessage('')
    try {
      await api.updateSettings({ accepting_offers: acceptingOffers })
      setMessage('Настройки сохранены')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setSaving(false)
    }
  }

  if (!user) return null

  return (
    <div className={styles.page}>
      <h1>Настройки</h1>

      {error && <div className="alert error">{error}</div>}
      {message && <div className="alert success">{message}</div>}

      <section className={styles.section}>
        <h2>Профиль</h2>
        <form onSubmit={handleProfile} className={styles.form}>
          <label>
            Отображаемое имя
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={user.username}
              maxLength={100}
            />
          </label>
          <div className={styles.meta}>
            <span>Email: {user.email}</span>
            <span>
              Username: @{user.username} —{' '}
              <Link to={`/s/${user.username}`}>ваша страница</Link>
            </span>
          </div>
          <button type="submit" disabled={saving}>
            {saving ? 'Сохранение...' : 'Сохранить профиль'}
          </button>
        </form>
      </section>

      <section className={styles.section}>
        <h2>Роль</h2>
        {user.role === 'streamer' ? (
          <p className={styles.roleBadge}>Вы стример — зрители могут предлагать вам видео</p>
        ) : (
          <div className={styles.becomeStreamer}>
            <p>Стать стримером, чтобы принимать предложения видео от зрителей</p>
            <button type="button" onClick={becomeStreamer} disabled={becomingStreamer}>
              {becomingStreamer ? '...' : 'Стать стримером'}
            </button>
          </div>
        )}
      </section>

      {user.role === 'streamer' && (
        <section className={styles.section}>
          <h2>Предложка</h2>
          {loadingSettings ? (
            <div className="page-loading">Загрузка...</div>
          ) : (
            <form onSubmit={handleSettings} className={styles.form}>
              <label className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={acceptingOffers}
                  onChange={(e) => setAcceptingOffers(e.target.checked)}
                />
                <span>Принимать предложения видео</span>
              </label>
              <button type="submit" disabled={saving}>
                {saving ? 'Сохранение...' : 'Сохранить'}
              </button>
            </form>
          )}
        </section>
      )}
    </div>
  )
}
