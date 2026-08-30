import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Offer, OfferStatus } from '../api/types'
import { OfferCard } from '../components/OfferCard'
import styles from './QueuePage.module.css'

const tabs: { value: OfferStatus; label: string }[] = [
  { value: 'pending', label: 'В очереди' },
  { value: 'watched', label: 'Просмотрено' },
  { value: 'skipped', label: 'Пропущено' },
  { value: 'rejected', label: 'Отклонено' },
]

export function QueuePage() {
  const [status, setStatus] = useState<OfferStatus>('pending')
  const [offers, setOffers] = useState<Offer[]>([])
  const [cursor, setCursor] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [acting, setActing] = useState<string | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)

  const load = useCallback(async (st: OfferStatus, next?: string) => {
    return api.listQueue({ status: st, limit: 20, cursor: next })
  }, [])

  async function loadMore() {
    if (!cursor || loadingMore) return
    setLoadingMore(true)
    try {
      const res = await load(status, cursor)
      setOffers((prev) => [...prev, ...res.items])
      setCursor(res.next_cursor)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setLoadingMore(false)
    }
  }

  useEffect(() => {
    setLoading(true)
    setError('')
    load(status)
      .then((res) => {
        setOffers(res.items)
        setCursor(res.next_cursor)
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка'))
      .finally(() => setLoading(false))
  }, [status, load])

  async function updateStatus(id: string, newStatus: OfferStatus) {
    setActing(id)
    try {
      const updated = await api.updateOfferStatus(id, newStatus)
      if (status === 'pending') {
        setOffers((prev) => prev.filter((o) => o.id !== id))
      } else {
        setOffers((prev) => prev.map((o) => (o.id === id ? updated : o)))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setActing(null)
    }
  }

  async function remove(id: string) {
    if (!confirm('Удалить оффер из очереди?')) return
    setActing(id)
    try {
      await api.deleteOffer(id)
      setOffers((prev) => prev.filter((o) => o.id !== id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка')
    } finally {
      setActing(null)
    }
  }

  return (
    <div>
      <header className={styles.header}>
        <h1>Очередь видео</h1>
        <p>Управляйте предложениями от зрителей</p>
      </header>

      <div className={styles.tabs}>
        {tabs.map((tab) => (
          <button
            key={tab.value}
            type="button"
            className={status === tab.value ? styles.active : ''}
            onClick={() => setStatus(tab.value)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {error && <div className="alert error">{error}</div>}

      {loading ? (
        <div className="page-loading">Загрузка очереди...</div>
      ) : offers.length === 0 ? (
        <div className="empty-state">
          <p>
            {status === 'pending' ? 'Очередь пуста' : 'Нет офферов с таким статусом'}
          </p>
        </div>
      ) : (
        <div className={styles.list}>
          {offers.map((offer) => (
            <OfferCard
              key={offer.id}
              offer={offer}
              actions={
                <>
                  {status === 'pending' && (
                    <>
                      <button
                        type="button"
                        className={`${styles.btn} ${styles.watched}`}
                        disabled={acting === offer.id}
                        onClick={() => updateStatus(offer.id, 'watched')}
                      >
                        ✓ Просмотрено
                      </button>
                      <button
                        type="button"
                        className={styles.btn}
                        disabled={acting === offer.id}
                        onClick={() => updateStatus(offer.id, 'skipped')}
                      >
                        Пропустить
                      </button>
                      <button
                        type="button"
                        className={`${styles.btn} ${styles.reject}`}
                        disabled={acting === offer.id}
                        onClick={() => updateStatus(offer.id, 'rejected')}
                      >
                        Отклонить
                      </button>
                    </>
                  )}
                  <button
                    type="button"
                    className={`${styles.btn} ${styles.delete}`}
                    disabled={acting === offer.id}
                    onClick={() => remove(offer.id)}
                  >
                    Удалить
                  </button>
                </>
              }
            />
          ))}
          {cursor && (
            <div className={styles.more}>
              <button type="button" onClick={loadMore} disabled={loadingMore}>
                {loadingMore ? 'Загрузка...' : 'Показать ещё'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
