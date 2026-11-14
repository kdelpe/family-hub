import styles from './SignInTree.module.css'
import { branches, grass, leaves, mountains, roots } from './signInTreeData'

// The mockup's tree is fully grown at ~4.7 s (last leaf's pop included); the owner wants ~2 s. This scales
// every delay, draw and pop; the sway and the leaf-colour cycle keep their period and stagger.
const growthSpeed = 2.35
const popDuration = 0.4
const grassDelay = 0.5
const grassDuration = 0.4
const rootDuration = 0.7
const swayDelay = 4.6

const leafColor = (index: number) => `var(--leaf-${index + 1})`
const seconds = (s: number) => `${Math.round(s * 1000) / 1000}s`
const atSpeed = (s: number) => seconds(s / growthSpeed)

export function SignInTree() {
  return (
    <svg aria-hidden="true" viewBox="-120 -239 723 889" className={styles.tree}>
      {(['back', 'front', 'ground'] as const).map((which) => {
        const [d, duration, delay] = mountains[which]
        return (
          <path
            key={which}
            d={d}
            pathLength={100}
            className={`${styles.draw} ${styles[which]}`}
            style={{ animationDuration: atSpeed(duration), animationDelay: atSpeed(delay) }}
          />
        )
      })}
      {grass.map((d) => (
        <path key={d} d={d} className={styles.grass} style={{ animationDuration: atSpeed(grassDuration), animationDelay: atSpeed(grassDelay) }} />
      ))}
      {roots.map(([d, width, delay]) => (
        <path
          key={d}
          d={d}
          pathLength={100}
          strokeWidth={width}
          className={`${styles.draw} ${styles.root}`}
          style={{ animationDuration: atSpeed(rootDuration), animationDelay: atSpeed(delay) }}
        />
      ))}
      <g className={styles.sway} style={{ animationDelay: atSpeed(swayDelay) }}>
        <g className={styles.branches}>
          {branches.map(([d, width, duration, delay]) => (
            <path
              key={d}
              d={d}
              pathLength={100}
              strokeWidth={width}
              className={`${styles.draw} ${styles.branch}`}
              style={{ animationDuration: atSpeed(duration), animationDelay: atSpeed(delay) }}
            />
          ))}
        </g>
        {leaves.map(([cx, cy, r, color, second, popDelay, cycleDelay]) => (
          <g key={`${cx},${cy}`} className={styles.leaf} style={{ animationDuration: atSpeed(popDuration), animationDelay: atSpeed(popDelay) }}>
            <circle cx={cx} cy={cy} r={r} style={{ fill: leafColor(color) }} />
            <circle
              cx={cx}
              cy={cy}
              r={r}
              className={styles.cycle}
              // Only the wait for the leaf to appear speeds up; its offset from the pop keeps the mockup's stagger.
              style={{ fill: leafColor(second), animationDelay: seconds(popDelay / growthSpeed + cycleDelay - popDelay) }}
            />
          </g>
        ))}
      </g>
    </svg>
  )
}
