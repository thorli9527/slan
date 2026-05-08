export function slug(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'x';
}

export function shortCodeFromEmail(email: string): string {
  const name = email.split('@')[0] || 'user';
  return slug(name).slice(0, 16) || 'user';
}
