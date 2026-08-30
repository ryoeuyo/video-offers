import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { StreamerPublic } from '../api/types'
import { StreamerCard } from '../components/StreamerCard'
import styles from './HomePage.module.css'

export function HomePage() {
  const [query, setQuery] = useState('')
  const [search, setSearch] = useState('')
  const [streamers, setStreamers] = useState<StreamerPublic[]>([])
  const [cursor, setCursor] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async (q: string, next?: string) => {
    const res = await api.listStreamers({ q: q || undefined, limit: 20, cursor: next })
    return res
  }, [])

  useEffect(() => {
    setLoading(true)
    setError('')
    load(search)
      .then((res) => {
        setStreamers(res.items)
        setCursor(res.next_cursor)
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка загрузки'))
      .finally(() => setLoading(false))
  }, [search, load])

  function handleSearch(e: FormEvent) {
    e.preventDefault()
    setSearch(query.trim())
  }

  async function loadMore() {
    if (!cursor || loadingMore) return
    setLoadingMore(true)
    try {
      const res = await load(search, cursor)
      setStreamers((prev) => [...prev, ...res.items])
      setCursor(res.next_cursor)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка загрузки')
    } finally {
      setLoadingMore(false)
    }
  }

  return (
    <div>
      <header className={styles.hero}>
        <h1>Предложи видео стримеру</h1>
        <p>Найди стримера и отправь ссылку на YouTube, Twitch или другой источник</p>
      </header>

      <form onSubmit={handleSearch} className={styles.search}>
        <input
          type="search"
          placeholder="Поиск по username..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <button type="submit">Найти</button>
      </form>

      {error && <div className="alert error">{error}</div>}

      {loading ? (
        <div className="page-loading">Загрузка стримеров...</div>
      ) : streamers.length === 0 ? (
        <div className="empty-state">
          <p>Стримеров пока нет</p>
          <span>Зарегистрируйтесь и включите роль стримера в настройках</span>
        </div>
      ) : (
        <>
          <div className={styles.list}>
            {streamers.map((s) => (
              <StreamerCard key={s.username} streamer={s} />
            ))}
          </div>
          {cursor && (
            <div className={styles.more}>
              <button type="button" onClick={loadMore} disabled={loadingMore}>
                {loadingMore ? 'Загрузка...' : 'Показать ещё'}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
