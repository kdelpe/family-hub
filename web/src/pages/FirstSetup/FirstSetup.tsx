import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'

import page from '../../styles/signInForm.module.css'
import styles from './FirstSetup.module.css'
import { sampleSetup } from './sampleSetup'

const steps = 3

export function FirstSetup() {
  const [step, setStep] = useState(0)
  const [bio, setBio] = useState(sampleSetup.bio)
  const [visible, setVisible] = useState<Record<string, boolean>>({})
  const heading = useRef<HTMLHeadingElement>(null)
  const moved = useRef(false)

  // The buttons stay put while the step above them changes, so focus moves to the new heading for screen readers.
  useEffect(() => {
    if (moved.current) {
      heading.current?.focus()
    }
  }, [step])

  const goTo = (next: number) => {
    moved.current = true
    setStep(next)
  }
  const last = step === steps - 1

  return (
    <>
      <title>Getting started — Pluviose-Monestime Family Hub</title>
      <main className={`${page.main} ${page.tallMain}`}>
        <div className={`${page.form} ${styles.column}`}>
          <div className={styles.progress}>
            <span className={styles.stepCount}>
              Step {step + 1} of {steps}
            </span>
            <div aria-hidden="true" className={styles.bars}>
              {Array.from({ length: steps }, (_, i) => (
                <span key={i} className={i <= step ? `${styles.bar} ${styles.done}` : styles.bar} />
              ))}
            </div>
          </div>

          {step === 0 && (
            <section aria-labelledby="fs-h0" className={`${styles.step} ${styles.about}`}>
              <h1 ref={heading} id="fs-h0" tabIndex={-1} className={`${page.title} ${styles.title}`}>
                Let the family see who you are
              </h1>
              <div className={styles.photoRow}>
                <span aria-hidden="true" className={styles.avatar}>
                  {sampleSetup.initials}
                </span>
                <div className={styles.photoActions}>
                  {/* Photos arrive with uploads in Phase 5. */}
                  <button type="button" aria-disabled="true" className={`pill ${styles.photoButton}`}>
                    Add a photo
                  </button>
                  <span className={page.hint}>You can do this later too.</span>
                </div>
              </div>
              <div className={page.field}>
                <label htmlFor="fs-bio" className={page.label}>
                  A line or two about you
                </label>
                <textarea id="fs-bio" rows={3} value={bio} onChange={(event) => setBio(event.target.value)} className={`${page.input} ${styles.bio}`} />
              </div>
            </section>
          )}

          {step === 1 && (
            <section aria-labelledby="fs-h1" className={`${styles.step} ${styles.reach}`}>
              <h1 ref={heading} id="fs-h1" tabIndex={-1} className={`${page.title} ${styles.title}`}>
                How can family reach you?
              </h1>
              <p className={styles.text}>
                Phone, email, address and social accounts all start as “Only me.” Turn on what you’d like the family to see.
              </p>
              {sampleSetup.contacts.map((contact) => {
                const on = visible[contact.id] === true
                return (
                  <div key={contact.id} className={styles.contact}>
                    <div className={styles.contactText}>
                      <span id={`fs-${contact.id}`} className={styles.contactLabel}>
                        {contact.label}
                      </span>
                      <span>{contact.value}</span>
                    </div>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={on}
                      aria-labelledby={`fs-${contact.id}`}
                      aria-describedby={`fs-${contact.id}-status`}
                      onClick={() => setVisible((current) => ({ ...current, [contact.id]: !current[contact.id] }))}
                      className={styles.switch}
                    >
                      <span aria-hidden="true" className={styles.track}>
                        <span className={styles.knob} />
                      </span>
                      <span id={`fs-${contact.id}-status`} className={styles.status}>
                        {on ? (contact.sharedOnRequest ? 'Family can ask' : 'Visible to family') : 'Only me'}
                      </span>
                    </button>
                  </div>
                )
              })}
              <span className={page.hint}>Administrators can see all information you enter.</span>
            </section>
          )}

          {step === 2 && (
            <section aria-labelledby="fs-h2" className={`${styles.step} ${styles.place}`}>
              <h1 ref={heading} id="fs-h2" tabIndex={-1} className={`${page.title} ${styles.title}`}>
                Here’s your place in the family
              </h1>
              <div className={styles.tree}>
                <span className={styles.generation}>Grandparents</span>
                <div className={styles.people}>
                  {sampleSetup.grandparents.map((name) => (
                    <span key={name} className={styles.grandparent}>
                      {name}
                    </span>
                  ))}
                </div>
                <span aria-hidden="true" className={styles.connector} />
                <span className={styles.generation}>Parents</span>
                <div className={styles.people}>
                  {sampleSetup.parents.map((name) => (
                    <span key={name} className={styles.parent}>
                      {name}
                    </span>
                  ))}
                </div>
                <span aria-hidden="true" className={styles.connector} />
                <span className={styles.you}>{sampleSetup.firstName} · You</span>
              </div>
              <p className={styles.text}>
                {sampleSetup.familyNote} Something look wrong? Let an admin know and they’ll fix it.
              </p>
            </section>
          )}

          <div className={styles.nav}>
            {step > 0 && (
              <button type="button" onClick={() => goTo(step - 1)} className={`pill ${page.secondary} ${styles.back}`}>
                Back
              </button>
            )}
            {last ? (
              <Link to="/" className={`pill ${page.primary} ${styles.next}`}>
                Go to my home page
              </Link>
            ) : (
              <button type="button" onClick={() => goTo(step + 1)} className={`pill ${page.primary} ${styles.next}`}>
                Continue
              </button>
            )}
          </div>
          {!last && (
            <button type="button" onClick={() => goTo(step + 1)} className={styles.skip}>
              Skip for now
            </button>
          )}
        </div>
      </main>
    </>
  )
}
