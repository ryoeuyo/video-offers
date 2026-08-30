import type { OfferStatus } from '../api/types'
import styles from './StatusBadge.module.css'

const labels: Record<OfferStatus, string> = {
  pending: 'В очереди',
  watched: 'Просмотрено',
  skipped: 'Пропущено',
  rejected: 'Отклонено',
}

export function StatusBadge({ status }: { status: OfferStatus }) {
  return <span className={`${styles.badge} ${styles[status]}`}>{labels[status]}</span>
}
