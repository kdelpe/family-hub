import { Link } from 'react-router'

import {
  type ContactMethod,
  type DirectoryEntry,
  isRequestable,
  kindOf,
  kindWords,
  platformLabel,
  type RequestKind,
  requestKinds,
} from '../../api/contacts'
import type { FamilySide } from '../../api/accessRequests'
import { AskButton } from '../../components/AskButton/AskButton'
import { CheckPill } from '../../components/CheckPill/CheckPill'
import { PhotoFade } from '../../components/PhotoFade/PhotoFade'
import { familyLines, sideTags } from '../../lib/familySides'
import { fullName, initials } from '../../lib/names'
import { type Book, type BookPage, letterOf, nameOf, pageNumber } from './bookPages'
import styles from './Members.module.css'

const sideClass: Record<FamilySide | 'both', string> = {
  first: styles.sideFirst!,
  second: styles.sideSecond!,
  both: styles.sideBoth!,
  married_in: styles.sideMarriedIn!,
}

const sideOrder: (FamilySide | 'both')[] = ['first', 'second', 'both', 'married_in']

export function TreeMark({ size, className }: { size: number; className: string }) {
  return (
    <svg aria-hidden="true" width={size} height={size} viewBox="0 0 80 80" className={className}>
      <g fill="none" stroke="currentColor" strokeLinecap="round">
        <path d="M40 70 V38" strokeWidth="3.2" />
        <path
          d="M40 52 C33 47 27 45 20 38 M40 52 C47 47 53 45 60 38 M40 44 C36 38 32 34 28 26 M40 44 C44 38 48 34 52 26 M40 38 V18"
          strokeWidth="2.2"
        />
        <path d="M30 70 C35 67 45 67 50 70" strokeWidth="1.6" />
      </g>
      <g fill="currentColor">
        <circle cx="20" cy="36" r="4.2" />
        <circle cx="60" cy="36" r="4.2" />
        <circle cx="27" cy="24" r="4.2" />
        <circle cx="53" cy="24" r="4.2" />
        <circle cx="40" cy="15" r="4.6" />
        <circle cx="33" cy="31" r="3" />
        <circle cx="47" cy="31" r="3" />
        <circle cx="14" cy="44" r="2.6" />
        <circle cx="66" cy="44" r="2.6" />
      </g>
    </svg>
  )
}

const flourish = (where: string) => (
  <svg aria-hidden="true" width="34" height="34" viewBox="0 0 34 34" className={`${styles.flourish} ${where}`}>
    <path d="M3 31 V12 C3 6 6 3 12 3 H31 M9 31 V15 C9 11 11 9 15 9 H31" />
    <circle cx="15" cy="15" r="2.2" />
  </svg>
)

const linkIcon = (
  <svg aria-hidden="true" width="13" height="13" viewBox="0 0 24 24" className={styles.linkIcon}>
    <path d="M10 14 L14 10 M8.5 16.5 L6.8 18.2 A3.5 3.5 0 0 1 1.8 13.2 L5 10 A3.5 3.5 0 0 1 10 10 M15.5 7.5 L17.2 5.8 A3.5 3.5 0 0 1 22.2 10.8 L19 14 A3.5 3.5 0 0 1 14 14" />
  </svg>
)

// The local part may hold "?" or "&", which would add recipients or headers; "@" stays as is for mail apps.
const mailto = (email: string) => {
  const at = email.lastIndexOf('@')
  if (at < 0) {
    return undefined
  }
  return `mailto:${encodeURIComponent(email.slice(0, at))}@${encodeURIComponent(email.slice(at + 1))}`
}

const inSentence: Record<RequestKind, string> = { phone: 'phone number', whatsapp: 'WhatsApp', address: 'address' }

// "Phone number, WhatsApp and address"
const onRequestWords = (kinds: RequestKind[]) => {
  const words = kinds.map((k) => inSentence[k])
  const list = words.length > 1 ? `${words.slice(0, -1).join(', ')} and ${words.at(-1)}` : (words[0] ?? '')
  return list.charAt(0).toUpperCase() + list.slice(1)
}

type Props = {
  book: Book
  index: number
  // Which corner the big letter sits in: the outer one.
  outer: 'left' | 'right'
  viewerId: string | null
  paused: boolean
  goTo: (index: number) => void
}

export function PageContent({ book, index, outer, viewerId, paused, goTo }: Props) {
  const page = book.pages[index]!
  const number = pageNumber(book, index)
  const person = nameOf(page)
  return (
    <>
      {person && (
        <span aria-hidden="true" className={`${styles.cornerLetter} ${outer === 'left' ? styles.atLeft : styles.atRight}`}>
          {letterOf(person.first_name)}
        </span>
      )}
      {number && <span className={styles.pageNumber}>{number}</span>}
      <Body page={page} book={book} viewerId={viewerId} paused={paused} goTo={goTo} />
    </>
  )
}

function Body({ page, book, viewerId, paused, goTo }: { page: BookPage } & Omit<Props, 'index' | 'outer'>) {
  switch (page.kind) {
    case 'person':
      return <PersonPage entry={page.living.entry} relation={page.living.relation} self={page.living.entry.id === viewerId} paused={paused} />
    case 'memorial': {
      const { person, relation } = page.remembered
      return (
        <div className={styles.memorialPage}>
          <Head person={person} relation={relation} side={person.side} paused={paused} remembered />
          <span className={styles.inMemory}>
            <svg aria-hidden="true" width="12" height="14" viewBox="0 0 10 12">
              <path d="M5 11 C1 8 1 3 5 1 C9 3 9 8 5 11 Z" />
            </svg>
            In loving memory
          </span>
          {person.bio_excerpt && <p className={styles.remembered}>“{person.bio_excerpt}”</p>}
          <div className={styles.memorialLinks}>
            <Link to={`/people/${person.id}`}>
              Open profile <span className="visually-hidden">of {fullName(person)}</span>›
            </Link>
          </div>
        </div>
      )
    }
    case 'cover':
      return (
        <div className={styles.cover}>
          <span aria-hidden="true" className={styles.hinge} />
          <span aria-hidden="true" className={styles.border} />
          <span aria-hidden="true" className={styles.borderInner} />
          {flourish(styles.topLeft!)}
          {flourish(styles.topRight!)}
          {flourish(styles.bottomLeft!)}
          {flourish(styles.bottomRight!)}
          <div className={styles.coverWords}>
            <TreeMark size={120} className={styles.gold!} />
            <span className={styles.coverTitle}>{`${familyLines.first}-${familyLines.second}`}</span>
            <span aria-hidden="true" className={styles.coverRule}>
              <span />
              <span className={styles.diamond} />
              <span />
            </span>
            <span className={styles.coverLabel}>Family members</span>
            <span className={styles.motto}>Lakay se lakay</span>
          </div>
          <span aria-hidden="true" className={styles.sheen} />
        </div>
      )
    case 'back':
      return (
        <div className={`${styles.cover} ${styles.backCover}`}>
          <span aria-hidden="true" className={styles.hingeBack} />
          <span aria-hidden="true" className={styles.borderBack} />
          <TreeMark size={64} className={styles.goldSoft!} />
        </div>
      )
    case 'endpaperFront':
      return (
        <div className={styles.endpaper}>
          <div className={styles.bookplate}>
            <TreeMark size={70} className={styles.accentMark!} />
            <span className={styles.kicker}>This book belongs to</span>
            <span className={styles.bookplateName}>{`the ${familyLines.first}-${familyLines.second} family`}</span>
            <span className={styles.bookplateNote}>Kept by everyone, for everyone.</span>
          </div>
        </div>
      )
    case 'endpaperBack':
      return (
        <div className={styles.endpaper}>
          <div className={styles.proverb}>
            <span className={styles.proverbKreyol}>Sa w plante, se li w rekòlte.</span>
            <span className={styles.proverbEnglish}>What you plant is what you harvest.</span>
          </div>
        </div>
      )
    case 'welcome':
      return (
        <div className={styles.textPage}>
          <span className={styles.kicker}>Welcome</span>
          <h2 className={styles.welcomeTitle}>Everyone in our family, in one book</h2>
          <p className={styles.welcomeIntro}>
            One page for each of us, A to Z, with how to reach the people who are with us. At the back, a section to
            remember those who came before.
          </p>
          <div className={styles.notesList}>
            <Note mark="@">Email and social accounts appear when someone chooses to share them.</Note>
            <Note mark="?">Phone numbers, WhatsApp and addresses are never printed here. Ask, and they decide.</Note>
            <Note mark="↻">Click or drag a page to turn it, or tap or slide along the tabs on the edge.</Note>
          </div>
          <div className={styles.ownPage}>
            Your own page is filled in from <Link to="/profile">My profile ›</Link>
          </div>
        </div>
      )
    case 'contents': {
      const everyone = [...book.living.map((l) => l.entry.side), ...book.remembered.map((r) => r.person.side)]
      return (
        <div className={styles.textPage}>
          <h2 className={styles.contentsTitle}>Inside</h2>
          <div className={styles.toc}>
            <Entry title="Welcome" page={pageNumber(book, book.welcome)!} onGo={() => goTo(book.welcome)} />
            {book.living.length > 0 && (
              <Entry
                title="Members, A to Z"
                page={pageNumber(book, book.membersStart)!}
                note={`${book.living.length} ${book.living.length === 1 ? 'person' : 'people'} with us`}
                onGo={() => goTo(book.membersStart)}
              >
                <div className={styles.letters}>
                  {book.letterStarts.map(({ letter, index }) => (
                    <button key={letter} type="button" className={`pill ${styles.letter}`} aria-label={`Go to ${letter}`} onClick={() => goTo(index)}>
                      {letter}
                    </button>
                  ))}
                </div>
              </Entry>
            )}
            <Entry
              title="In Memory"
              page={pageNumber(book, book.memoryStart)!}
              note={`${book.remembered.length} remembered`}
              onGo={() => goTo(book.memoryStart)}
            />
          </div>
          <div className={styles.sideCounts}>
            {sideOrder.map((side) => (
              <span key={side} className={`${styles.sideCount} ${sideClass[side]}`}>
                {sideTags[side]} · {everyone.filter((s) => s === side).length}
              </span>
            ))}
          </div>
        </div>
      )
    }
    case 'divider':
      return (
        <div className={styles.divider}>
          <svg aria-hidden="true" width="150" height="60" viewBox="0 0 150 60" className={styles.branch}>
            <path d="M10 40 C50 26 100 26 140 40" className={styles.stem} />
            <g>
              <path d="M35 33 C30 22 38 16 42 20 C44 26 40 31 35 33 Z" />
              <path d="M55 29 C52 17 61 12 64 17 C65 23 60 28 55 29 Z" />
              <path d="M95 29 C98 17 89 12 86 17 C85 23 90 28 95 29 Z" />
              <path d="M115 33 C120 22 112 16 108 20 C106 26 110 31 115 33 Z" />
              <path d="M45 36 C40 46 48 51 52 47 C53 42 50 38 45 36 Z" opacity=".7" />
              <path d="M105 36 C110 46 102 51 98 47 C97 42 100 38 105 36 Z" opacity=".7" />
            </g>
            <circle cx="75" cy="27" r="4" className={styles.berry} />
          </svg>
          <h2 className={styles.dividerTitle}>In Memory</h2>
          <p className={styles.dividerText}>
            Those who came before us. Their pages hold what we remember. Open a profile to see their place in the tree,
            or share a memory of your own.
          </p>
          <span className={styles.kicker}>{book.remembered.length} remembered</span>
        </div>
      )
    case 'notes':
      return (
        <div className={styles.notesPage}>
          <TreeMark size={56} className={styles.faintMark!} />
          <span className={styles.notesTitle}>Notes</span>
        </div>
      )
    case 'empty':
      return null
  }
}

function Note({ mark, children }: { mark: string; children: string }) {
  return (
    <div className={styles.note}>
      <span aria-hidden="true" className={styles.noteMark}>
        {mark}
      </span>
      <span>{children}</span>
    </div>
  )
}

function Entry({ title, page, note, onGo, children }: { title: string; page: string; note?: string; onGo: () => void; children?: React.ReactNode }) {
  return (
    <div className={styles.entry}>
      <button type="button" className={styles.entryButton} onClick={onGo}>
        <span className={styles.entryTitle}>{title}</span>
        <span aria-hidden="true" className={styles.leader} />
        <span className={styles.entryPage}>
          <span className="visually-hidden">, page</span> {page}
        </span>
      </button>
      {note && <span className={styles.entryNote}>{note}</span>}
      {children}
    </div>
  )
}

function Head({
  person,
  relation,
  side,
  paused,
  remembered = false,
}: {
  person: { first_name: string; last_name: string | null }
  relation: string | null
  side: DirectoryEntry['side']
  paused: boolean
  remembered?: boolean
}) {
  return (
    <div className={styles.head}>
      <span aria-hidden="true" className={`${styles.portrait} ${remembered ? styles.sepia : ''}`}>
        {initials(person)}
        <PhotoFade person={person} paused={paused} />
      </span>
      <h2 className={styles.name}>{fullName(person)}</h2>
      {relation && <span className={styles.relation}>{relation}</span>}
      {side && (
        <span className={styles.chips}>
          <span className={`${styles.chip} ${sideClass[side]}`}>{sideTags[side]}</span>
        </span>
      )}
    </div>
  )
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className={styles.section}>
      <span className={styles.sectionLabel}>{label}</span>
      <div className={styles.sectionBody}>{children}</div>
    </div>
  )
}

function PersonPage({ entry, relation, self, paused }: { entry: DirectoryEntry; relation: string | null; self: boolean; paused: boolean }) {
  const rows = entry.contacts.filter((c): c is ContactMethod => !isRequestable(c))
  const family = rows.filter((c) => c.visibility === 'family')
  const emails = family.filter((c) => c.kind === 'email')
  const socials = family.filter((c) => c.kind === 'social' || c.kind === 'other')
  // The values stay on the profile: the book never prints them (design-notes §4.3).
  const sharedWithYou = requestKinds.filter((k) => rows.some((c) => c.shared_with_you && kindOf(c) === k))
  // An admin gets these rows in full and can't ask, so they only learn what's there to ask for.
  const onRequest = requestKinds.filter((k) => rows.some((c) => c.visibility === 'on_request' && !c.shared_with_you && kindOf(c) === k))
  const asks = entry.contacts.filter(isRequestable).sort((a, b) => requestKinds.indexOf(a.kind) - requestKinds.indexOf(b.kind))
  const shown = emails.length + socials.length + sharedWithYou.length + onRequest.length + asks.length > 0

  return (
    <div className={styles.personPage}>
      <Head person={entry} relation={self ? 'You' : relation} side={entry.side} paused={paused} />
      <div aria-hidden="true" className={styles.rule} />
      <div className={styles.reach}>
        {self ? (
          <div className={styles.yourPage}>
            <span className={styles.yourPageTitle}>This is your page.</span>
            <span className={styles.boxText}>
              Family see only what you choose to share. Phone numbers and addresses stay private until you say yes.
            </span>
            <Link to="/profile" className={styles.boxLink}>
              Edit in My profile ›
            </Link>
          </div>
        ) : (
          <>
            {emails.length > 0 && (
              <Section label="Email">
                {emails.map((c) => (
                  <span key={c.id} className={styles.emailRow}>
                    <span className={styles.emailValue}>{c.value}</span>
                    {mailto(c.value) && (
                      <a
                        href={mailto(c.value)}
                        className={`pill tap ${styles.emailButton}`}
                        aria-label={emails.length > 1 ? `Email ${entry.first_name} at ${c.value}` : `Email ${entry.first_name}`}
                      >
                        Email
                      </a>
                    )}
                  </span>
                ))}
              </Section>
            )}
            {socials.length > 0 && (
              <Section label="Find them on">
                {socials.map((c) => {
                  // The platform's name, so a label like "Old account" never hides where the link goes.
                  const platformName = c.kind === 'other' || c.platform === 'other' ? (c.label ?? 'Other') : platformLabel(c.platform)
                  // A value the server won't link (an "Other" account) still shows, as text.
                  const label = c.href ? platformName : `${platformName}: ${c.value}`
                  return c.href ? (
                    <a key={c.id} href={c.href} target="_blank" rel="noopener noreferrer" aria-label={`${entry.first_name} on ${label}`} className={`pill tap ${styles.social}`}>
                      {linkIcon}
                      {label}
                    </a>
                  ) : (
                    <span key={c.id} className={styles.social}>
                      {linkIcon}
                      {label}
                    </span>
                  )
                })}
              </Section>
            )}
            {(asks.length > 0 || sharedWithYou.length > 0 || onRequest.length > 0) && (
              <Section label="Ask to share">
                {sharedWithYou.map((k) => (
                  <CheckPill key={k} size="small" to={`/people/${entry.id}`}>
                    {kindWords[k]} shared with you
                  </CheckPill>
                ))}
                {asks.map((c) => (
                  <AskButton key={c.kind} personId={entry.id} kind={c.kind} asked={c.my_request_status === 'requested'} size="small" />
                ))}
                {onRequest.length > 0 && <span className={styles.onRequest}>{onRequestWords(onRequest)} on request</span>}
              </Section>
            )}
            {!entry.has_account && !shown && (
              <div className={styles.notOnHub}>Not on the hub yet, so there’s nothing to share here.</div>
            )}
          </>
        )}
      </div>
      <Link to={`/people/${entry.id}`} className={styles.openProfile}>
        Open profile <span className="visually-hidden">of {fullName(entry)}</span>›
      </Link>
    </div>
  )
}
