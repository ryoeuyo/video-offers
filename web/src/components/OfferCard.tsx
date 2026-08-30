import type { ReactNode } from 'react'
import type { Offer } from '../api/types'
import { StatusBadge } from './StatusBadge'
import styles from './OfferCard.module.css'

function formatDuration(seconds?: number): string | null {
  if (!seconds) return null
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function thumbnailSrc(offer: Offer): string | null {
  if (offer.thumbnail_url) return offer.thumbnail_url
  if (offer.provider === 'youtube' && offer.external_id) {
    return `https://i.ytimg.com/vi/${offer.external_id}/hqdefault.jpg`
  }
  return null
}

function providerLabel(provider: string): string {
  const map: Record<string, string> = {
    youtube: 'YouTube',
    twitch: 'Twitch',
    vk: 'VK',
    other: 'Ссылка',
  }
  return map[provider] ?? provider
}

function displayTitle(offer: Offer): string {
  if (offer.title.trim()) return offer.title
  return `${providerLabel(offer.provider)} видео`
}

interface OfferCardProps {
  offer: Offer
  actions?: ReactNode
}

export function OfferCard({ offer, actions }: OfferCardProps) {
  const title = displayTitle(offer)
  const duration = formatDuration(offer.duration_seconds)
  const thumb = thumbnailSrc(offer)

  return (
    <article className={styles.card}>
      <a
        href={offer.url}
        target="_blank"
        rel="noopener noreferrer"
        className={styles.thumbWrap}
      >
        {thumb ? (
          <img src={thumb} alt="" className={styles.thumb} loading="lazy" />
        ) : (
          <div className={styles.thumbPlaceholder}>
            <span>{providerLabel(offer.provider)}</span>
          </div>
        )}
        {duration && <span className={styles.duration}>{duration}</span>}
      </a>

      <div className={styles.body}>
        <div className={styles.meta}>
          <span className={styles.provider}>{providerLabel(offer.provider)}</span>
          <StatusBadge status={offer.status} />
        </div>

        <a
          href={offer.url}
          target="_blank"
          rel="noopener noreferrer"
          className={styles.title}
        >
          {title}
        </a>

        {offer.comment && <p className={styles.comment}>{offer.comment}</p>}

        <time className={styles.time} dateTime={offer.created_at}>
          {new Date(offer.created_at).toLocaleString('ru-RU')}
        </time>

        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
    </article>
  )
}
