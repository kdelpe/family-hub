import type { FamilySummary } from '../../api/family'
import styles from './FamilyStrip.module.css'

// The family at a glance, above the tree (design-notes §4.2).
export function FamilyStrip({ summary }: { summary: FamilySummary }) {
  const earliest = summary.earliest_record
  return (
    <div className={styles.strip}>
      <span className={`${styles.stat} ${styles.statFirst}`}>
        <strong>{summary.people}</strong> {summary.people === 1 ? 'person' : 'people'}
      </span>
      <span className={styles.stat}>
        <strong>{summary.generations}</strong> {summary.generations === 1 ? 'generation' : 'generations'}
      </span>
      {earliest && (
        <span className={styles.stat}>
          Earliest record: {earliest.circa ? 'c. ' : ''}
          {earliest.year}
          {earliest.place ? `, ${earliest.place}` : ''}
        </span>
      )}
    </div>
  )
}
