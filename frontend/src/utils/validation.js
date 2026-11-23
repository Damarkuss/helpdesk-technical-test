// Client-side checks mirror the server's rules so people get instant feedback;
// the server stays the source of truth and its field errors are shown too.

export function isEmail(value) {
  return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(value.trim())
}

/** Only follow same-site redirect targets after login (blocks "//evil.example" style open redirects). */
export function safeRedirect(target) {
  return typeof target === 'string' && target.startsWith('/') && !target.startsWith('//') ? target : null
}
