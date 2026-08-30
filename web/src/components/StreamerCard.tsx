import { Link } from 'react-router-dom'
import type { StreamerPublic } from '../api/types'
import styles from './StreamerCard.module.css'

function initials(name: string, username: string): string {
  const source = name || username
  return source.slice(0, 2).toUpperCase()
}

export function StreamerCard({ streamer }: { streamer: StreamerPublic }) {
  const display = streamer.display_name || streamer.username

  return (
    <Link to={`/s/${streamer.username}`} className={styles.card}>
      <div className={styles.avatar}>
        {streamer.avatar_url ? (
          <img src={streamer.avatar_url} alt="" />
        ) : (
          <span>{initials(streamer.display_name, streamer.username)}</span>
        )}
      </div>
      <div className={styles.body}>
        <div className={styles.name}>{display}</div>
        <div className={styles.username}>@{streamer.username}</div>
      </div>
      <span
        className={`${styles.status} ${streamer.accepting_offers ? styles.open : styles.closed}`}
      >
        {streamer.accepting_offers ? 'Принимает' : 'Закрыто'}
      </span>
    </Link>
  )
}
