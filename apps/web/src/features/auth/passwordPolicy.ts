export const passwordRequirements =
  'Password must contain at least 8 characters, one uppercase letter, and one number.'

export function validNewPassword(password: string): boolean {
  return Array.from(password).length >= 8 && /\p{Lu}/u.test(password) && /\p{Nd}/u.test(password)
}
