import * as HoverCard from '@radix-ui/react-hover-card'
import { m } from 'motion/react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { fullName, initials } from '../../lib/names'
import { useRelations } from '../../lib/useRelations'
import { useSignedIn } from '../../lib/useSignedIn'
import styles from './PersonHoverCard.module.css'

type Named = { id: string; first_name: string; last_name: string | null }

// Mouse only, as Radix has it: the name it wraps is still the link to their profile for touch and keyboard.
export function PersonHoverCard({ person, children }: { person: Named; children: ReactNode }) {
  return (
    <HoverCard.Root openDelay={250} closeDelay={120}>
      <HoverCard.Trigger asChild>{children}</HoverCard.Trigger>
      <HoverCard.Portal>
        <HoverCard.Content sideOffset={8} collisionPadding={16} asChild>
          <m.div className={styles.card} initial={{ opacity: 0, y: -6, scale: 0.97 }} animate={{ opacity: 1, y: 0, scale: 1 }} transition={{ duration: 0.16 }}>
            <Card person={person} />
          </m.div>
        </HoverCard.Content>
      </HoverCard.Portal>
    </HoverCard.Root>
  )
}

function Card({ person }: { person: Named }) {
  const { user } = useSignedIn()
  const viewerId = user?.person_id ?? null
  const { relations } = useRelations(viewerId)
  const relation = person.id === viewerId ? 'You' : relations?.get(person.id)
  return (
    <>
      <div className={styles.who}>
        <span aria-hidden="true" className={styles.avatar}>
          {initials(person)}
        </span>
        <span className={styles.words}>
          <strong>{fullName(person)}</strong>
          {relation && <span className={styles.relation}>{relation}</span>}
        </span>
      </div>
      <div className={styles.links}>
        <Link to={`/people/${person.id}`} className={styles.link}>
          Profile
        </Link>
        <Link to={`/tree?person=${encodeURIComponent(person.id)}`} className={styles.link}>
          In the tree
        </Link>
      </div>
    </>
  )
}
