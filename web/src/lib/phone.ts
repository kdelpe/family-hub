// A US number reads "111-555-0000" (or "1-111-555-0000") once its 10 digits are in. Shorter or longer ones,
// or one with a +, are another country's, so they stay as typed: their grouping differs.
export function formatPhone(typed: string): string {
  const digits = typed.replace(/\D/g, '')
  const usOne = digits.length === 11 && digits.startsWith('1')
  if (typed.includes('+') || /[^\d\s().-]/.test(typed)) {
    return typed
  }
  // A digit typed past a formatted US number: not one after all, so the dashes added for it come out.
  if (digits.length > 10 && !usOne && /^\d{3}-\d{3}-\d{5,}$/.test(typed)) {
    return digits
  }
  if (digits.length !== 10 && !usOne) {
    return typed
  }
  const local = usOne ? digits.slice(1) : digits
  const grouped = `${local.slice(0, 3)}-${local.slice(3, 6)}-${local.slice(6)}`
  return usOne ? `1-${grouped}` : grouped
}
