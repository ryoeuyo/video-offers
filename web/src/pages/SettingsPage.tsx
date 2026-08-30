import { type FormEvent, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, ApiClientError } from '../api/client'
import type { TwitchLink } from '../api/types'
import { useAuth } from '../context/AuthContext'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const { user, refreshUser } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const [displayName, setDisplayName] = useState('')
  const [acceptingOffers, setAcceptingOffers] = useState(true)
  const [requireTwitchSender, setRequireTwitchSender] = useState(false)
  const [requireFollow, setRequireFollow] = useState(false)
  const [followAgeDays, setFollowAgeDays] = useState(0)
  const [requireSubscription, setRequireSubscription] = useState(false)
  const [accountAgeDays, setAccountAgeDays] = useState(0)
  const [loadingSettings, setLoadingSettings] = useState(false)
  const [saving, setSaving] = useState(false)
  const [becomingStreamer, setBecomingStreamer] = useState(false)
  const [twitch, setTwitch] = useState<TwitchLink | null>(null)
  const [twitchAvailable, setTwitchAvailable] = useState<boolean | null>(null)
  const [twitchBusy, setTwitchBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (user) {
      setDisplayName(user.display_name)
    }
  }, [user])

  useEffect(() => {
    if (searchParams.get('twitch') === 'linked') {
      setMessage('Twitch привязан')
      searchParams.delete('twitch')
      setSearchParams(searchParams, { replace: true })
    }
  }, [searchParams, setSearchParams])

  useEffect(() => {
    api
      .getTwitch()
      .then((link) => {
        setTwitchAvailable(true)
        setTwitch(link)
      })
      .catch((err) => {
        if (err instanceof ApiClientError && err.code === 'route_not_found') {
          setTwitchAvailable(false)
          setTwitch(null)
          return
        }
        setTwitchAvailable(true)
        setTwitch({ linked: false })
      })
  }, [])

  useEffect(() => {
    if (user?.role !== 'streamer') return
    setLoadingSettings(true)
    api
      .getSettings()
      .then((s) => {
        setAcceptingOffers(s.accepting_offers)
        setRequireTwitchSender(s.require_twitch_sender)
        setRequireFollow(s.require_follow)
        setFollowAgeDays(secondsToDays(s.min_follow_age_seconds))
        setRequireSubscription(s.require_subscription)
        setAccountAgeDays(secondsToDays(s.min_account_age_seconds))
      })
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
      setMessage(
        twitch?.linked
          ? 'Вы стали стримером! Настройте приём предложений ниже.'
          : 'Вы стали стримером. Привяжите Twitch, чтобы включить приём офферов.',
      )
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
      await api.updateSettings({
        accepting_offers: acceptingOffers,
        min_account_age_seconds: daysToSeconds(accountAgeDays),
        require_twitch_sender: requireTwitchSender,
        require_follow: requireFollow,
        min_follow_age_seconds: daysToSeconds(followAgeDays),
        require_subscription: requireSubscription,
      })
      setMessage('Настройки сохранены')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setSaving(false)
    }
  }

  async function connectTwitch() {
    setTwitchBusy(true)
    setError('')
    try {
      const { url } = await api.connectTwitch()
      window.location.href = url
    } catch (err) {
      if (err instanceof ApiClientError && err.code === 'route_not_found') {
        setError(
          'Twitch не настроен на сервере. Добавьте TWITCH_CLIENT_ID и TWITCH_CLIENT_SECRET в .env и перезапустите API.',
        )
      } else {
        setError(err instanceof Error ? err.message : 'Ошибка')
      }
      setTwitchBusy(false)
    }
  }

  async function unlinkTwitch() {
    if (!confirm('Отвязать Twitch?')) return
    setTwitchBusy(true)
    setError('')
    setMessage('')
    try {
      await api.unlinkTwitch()
      setTwitch({ linked: false })
      setMessage('Twitch отвязан')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setTwitchBusy(false)
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
        <h2>Twitch</h2>
        {twitchAvailable === false ? (
          <p className={styles.hint}>
            Интеграция Twitch выключена на сервере. Заполните{' '}
            <code>TWITCH_CLIENT_ID</code> и <code>TWITCH_CLIENT_SECRET</code> в{' '}
            <code>.env</code> и перезапустите <code>make run</code>.
          </p>
        ) : twitch?.linked ? (
          <div className={styles.twitchLinked}>
            <p>
              Привязан как <strong>@{twitch.login}</strong>
              {twitch.display_name && twitch.display_name !== twitch.login
                ? ` (${twitch.display_name})`
                : ''}
            </p>
            <button
              type="button"
              className={styles.dangerBtn}
              onClick={unlinkTwitch}
              disabled={twitchBusy}
            >
              Отвязать
            </button>
          </div>
        ) : (
          <div className={styles.becomeStreamer}>
            <p>
              {user.role === 'streamer'
                ? 'Для приёма офферов нужен привязанный Twitch'
                : 'Привяжите Twitch, чтобы стримеры могли проверять follow и подписку'}
            </p>
            <button type="button" onClick={connectTwitch} disabled={twitchBusy}>
              {twitchBusy ? '...' : 'Привязать Twitch'}
            </button>
          </div>
        )}
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
                  disabled={twitchAvailable === true && !twitch?.linked}
                />
                <span>Принимать предложения видео</span>
              </label>
              {twitchAvailable === true && !twitch?.linked && (
                <p className={styles.hint}>Сначала привяжите Twitch</p>
              )}

              <label>
                Минимальный возраст аккаунта OfferBox (дни)
                <input
                  type="number"
                  min={0}
                  value={accountAgeDays}
                  onChange={(e) => setAccountAgeDays(Number(e.target.value) || 0)}
                />
              </label>

              <label className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={requireTwitchSender}
                  onChange={(e) => setRequireTwitchSender(e.target.checked)}
                  disabled={!twitch?.linked}
                />
                <span>Только с привязанным Twitch</span>
              </label>
              <label className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={requireFollow}
                  onChange={(e) => setRequireFollow(e.target.checked)}
                  disabled={!twitch?.linked}
                />
                <span>Только фолловеры канала</span>
              </label>
              <label>
                Минимальный возраст follow (дни, 0 — без ограничения)
                <input
                  type="number"
                  min={0}
                  value={followAgeDays}
                  onChange={(e) => setFollowAgeDays(Number(e.target.value) || 0)}
                  disabled={!twitch?.linked}
                />
              </label>
              <label className={styles.toggle}>
                <input
                  type="checkbox"
                  checked={requireSubscription}
                  onChange={(e) => setRequireSubscription(e.target.checked)}
                  disabled={!twitch?.linked || twitch?.has_subscription_scope === false}
                />
                <span>Только подписчики канала</span>
              </label>
              {twitch?.linked && twitch.has_subscription_scope === false && (
                <p className={styles.hint}>
                  Перепривяжите Twitch, чтобы проверять подписки (нужен scope channel:read:subscriptions)
                </p>
              )}
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

function secondsToDays(seconds: number): number {
  if (!seconds) return 0
  return Math.round(seconds / 86400)
}

function daysToSeconds(days: number): number {
  return Math.max(0, Math.floor(days)) * 86400
}
