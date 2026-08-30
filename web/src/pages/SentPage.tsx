import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Offer } from '../api/types'
import { OfferCard } from '../components/OfferCard'
import styles from './SentPage.module.css'

export function SentPage() {
  const [offers, setOffers] = useState<Offer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [acting, setActing] = useState<string | null>(null)

  const load = useCallback(async () => {
    return api.listSent({ limit: 50 })
  }, [])

  useEffect(() => {
    setLoading(true)
    load()
      .then((res) => setOffers(res.items))
      .catch((err) => setError(err instanceof Error ? err.message : 'Ошибка'))
      .finally(() => setLoading(false))
  }, [load])

  async function revoke(id: string) {
    if (!confirm('Отозвать предложение?')) return
    setActing(id)
    try {
      await api.revokeSent(id)
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
        <h1>Мои предложения</h1>
        <p>Видео, которые вы отправили стримерам</p>
      </header>

      {error && <div className="alert error">{error}</div>}

      {loading ? (
        <div className="page-loading">Загрузка...</div>
      ) : offers.length === 0 ? (
        <div className="empty-state">
          <p>Вы ещё ничего не предлагали</p>
          <span>Найдите стримера на главной и отправьте ссылку</span>
        </div>
      ) : (
        <div className={styles.list}>
          {offers.map((offer) => (
            <OfferCard
              key={offer.id}
              offer={offer}
              actions={
                offer.status === 'pending' ? (
                  <button
                    type="button"
                    className={styles.revoke}
                    disabled={acting === offer.id}
                    onClick={() => revoke(offer.id)}
                  >
                    Отозвать
                  </button>
                ) : undefined
              }
            />
          ))}
        </div>
      )}
    </div>
  )
}
