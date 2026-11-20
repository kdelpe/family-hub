// The server's limits (internal/auth/password.go).
export const minPasswordLength = 8
export const maxPasswordLength = 128

// Counted as the server counts: characters after NFC normalization, not bytes or UTF-16 units.
export const passwordLength = (password: string) => [...password.normalize('NFC')].length

export type Strength = { level: 0 | 1 | 2 | 3 | 4; label: string }

// A rough guide from length and variety; the server's common-password check has the final say.
export function passwordStrength(raw: string): Strength {
  const length = passwordLength(raw)
  const password = raw.normalize('NFC')
  if (length === 0) {
    return { level: 0, label: '' }
  }
  if (length < minPasswordLength) {
    return { level: 1, label: 'Too short' }
  }
  if (length > maxPasswordLength) {
    return { level: 1, label: 'Too long' }
  }
  // Unicode classes: an accented letter (è, É) is a letter, not a symbol.
  const kinds = [/\p{Ll}/u, /\p{Lu}/u, /\p{N}/u, /[^\p{L}\p{M}\p{N}]/u].filter((kind) => kind.test(password)).length
  if (length >= 16 || (length >= 12 && kinds >= 3)) {
    return { level: 4, label: 'Strong' }
  }
  if (length >= 12 || (length >= 10 && kinds >= 3)) {
    return { level: 3, label: 'Good' }
  }
  return { level: 2, label: 'Fair' }
}

// The plain-language rule a refused password broke (design-notes §3.3), or null for any other failure.
export function passwordRule(code: string | null): string | null {
  switch (code) {
    case 'password_too_short':
      return `Use at least ${minPasswordLength} characters.`
    case 'password_too_long':
      return `Use at most ${maxPasswordLength} characters.`
    case 'password_too_common':
      return 'That password is too common — try a short phrase.'
    default:
      return null
  }
}

// The length rule a password breaks before it's sent, as the server's code; null when it fits.
export function lengthRefusal(password: string): 'password_too_short' | 'password_too_long' | null {
  const length = passwordLength(password)
  return length < minPasswordLength ? 'password_too_short' : length > maxPasswordLength ? 'password_too_long' : null
}
