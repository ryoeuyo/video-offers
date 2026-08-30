import { type FormEvent, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, ApiClientError } from '../api/client'
import type { OfferRules, StreamerPublic } from '../api/types'
import { useAuth } from '../context/AuthContext'
import styles from './StreamerPage.module.css'

export function StreamerPage() {
  const { username } = useParams<{ username: string }>()
  const { user } = useAuth()
  const [streamer, setStreamer] = useState<StreamerPublic | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [url, setUrl] = useState('')
  const [comment, setComment] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState(false)
  const [formError, setFormError] = useState('')
  const [formErrorCode, setFormErrorCode] = useState('')

  useEffect(() => {
    if (!username) return
    setLoading(true)
    api
      .getStreamer(username)
      .then(setStreamer)
      .catch((err) => setError(err instanceof Error ? err.message : 'Стример не найден'))
      .finally(() => setLoading(false))
  }, [username])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!username) return
    setFormError('')
    setFormErrorCode('')
    setSuccess(false)
    setSubmitting(true)
    try {
      await api.createOffer(username, url.trim(), comment.trim())
      setUrl('')
      setComment('')
      setSuccess(true)
    } catch (err) {
      if (err instanceof ApiClientError) {
        setFormError(err.message)
        setFormErrorCode(err.code)
      } else {
        setFormError(err instanceof Error ? err.message : 'Не удалось отправить')
        setFormErrorCode('')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <div className="page-loading">Загрузка...</div>
  if (error || !streamer) {
    return (
      <div className="empty-state">
        <p>{error || 'Стример не найден'}</p>
        <Link to="/">← К списку</Link>
      </div>
    )
  }

  const display = streamer.display_name || streamer.username
  const isOwn = user?.username === streamer.username

  return (
    <div className={styles.page}>
      <div className={styles.profile}>
        <div className={styles.avatar}>
          {streamer.avatar_url ? (
            <img src={streamer.avatar_url} alt="" />
          ) : (
            display.slice(0, 2).toUpperCase()
          )}
        </div>
        <div>
          <h1>{display}</h1>
          <p className={styles.username}>
            @{streamer.username}
            {streamer.twitch_login && (
              <>
                {' · '}
                <a
                  href={`https://twitch.tv/${streamer.twitch_login}`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  twitch.tv/{streamer.twitch_login}
                </a>
              </>
            )}
          </p>
          <span
            className={`${styles.badge} ${streamer.accepting_offers ? styles.open : styles.closed}`}
          >
            {streamer.accepting_offers ? 'Принимает предложения' : 'Приём закрыт'}
          </span>
        </div>
      </div>

      {isOwn && (
        <div className="alert info">
          Это ваша страница.{' '}
          <Link to="/queue">Перейти к очереди →</Link>
        </div>
      )}

      {!user ? (
        <div className={styles.authPrompt}>
          <p>Войдите, чтобы предложить видео</p>
          <Link to="/login" className={styles.cta}>
            Войти
          </Link>
        </div>
      ) : !streamer.accepting_offers ? (
        <div className="alert warn">Стример временно не принимает предложения</div>
      ) : (
        <form onSubmit={handleSubmit} className={styles.form}>
          <h2>Предложить видео</h2>
          <OfferRulesList rules={streamer.offer_rules} />
          {formErrorCode === 'twitch_not_linked' && (
            <div className="alert warn">
              <Link to="/settings">Привяжите Twitch</Link>, чтобы отправлять офферы этому стримеру.
            </div>
          )}

          <label>
            Ссылка на видео
            <input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://youtube.com/watch?v=..."
              required
            />
          </label>

          <label>
            Комментарий <span className={styles.optional}>(необязательно)</span>
            <textarea
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder="Почему стоит посмотреть?"
              maxLength={500}
              rows={3}
            />
          </label>

          {formError && <div className="alert error">{formError}</div>}
          {success && (
            <div className="alert success">
              Видео отправлено!{' '}
              <Link to="/sent">Мои предложения →</Link>
            </div>
          )}

          <button type="submit" disabled={submitting}>
            {submitting ? 'Отправка...' : 'Отправить'}
          </button>
        </form>
      )}
    </div>
  )
}

function OfferRulesList({ rules }: { rules?: OfferRules }) {
  if (!rules) return null
  const items: string[] = []
  if (rules.min_account_age_seconds > 0) {
    items.push(`Аккаунт OfferBox старше ${Math.round(rules.min_account_age_seconds / 86400)} дн.`)
  }
  if (rules.require_twitch_sender) items.push('Привязанный Twitch')
  if (rules.require_follow) items.push('Фоллов на канал')
  if (rules.min_follow_age_seconds > 0) {
    items.push(`Фоллов старше ${Math.round(rules.min_follow_age_seconds / 86400)} дн.`)
  }
  if (rules.require_subscription) items.push('Подписка на канал')
  if (items.length === 0) return null
  return (
    <ul className={styles.rules}>
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  )
}
