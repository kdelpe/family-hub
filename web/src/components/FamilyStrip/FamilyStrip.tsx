import type { FamilySummary } from '../../api/family'
import styles from './FamilyStrip.module.css'

// The family at a glance, one quiet line in the tree's toolbar (design-notes §4.2).
export function FamilyStrip({ summary }: { summary: FamilySummary }) {
  const earliest = summary.earliest_record
  return (
    <p className={styles.strip}>
      <b>{summary.people}</b> {summary.people === 1 ? 'person' : 'people'} · <b>{summary.generations}</b>{' '}
      {summary.generations === 1 ? 'generation' : 'generations'}
      {earliest && ` · since ${earliest.circa ? 'c. ' : ''}${earliest.year}`}
    </p>
  )
}
