export function bytes(n: number, digits = 1): string {
  if (!isFinite(n) || n <= 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  const d = i === 0 ? 0 : n >= 100 ? 0 : n >= 10 ? Math.min(digits, 1) : digits
  return `${n.toFixed(d)} ${u[i]}`
}

export function rate(n: number): string {
  return n < 1024 ? '0 KB/s' : `${bytes(n)}/s`
}

export function pct(n: number, d = 0): string {
  return `${(n ?? 0).toFixed(d)}%`
}

export function count(n: number): string {
  return new Intl.NumberFormat().format(Math.round(n))
}

export function ago(ms: number): string {
  if (!ms) return ''
  const s = (Date.now() - ms) / 1000
  if (s < 60) return 'just now'
  const units: [number, string][] = [[60, 'minute'], [3600, 'hour'], [86400, 'day'], [2592000, 'month'], [31536000, 'year']]
  let label = ''
  for (let i = units.length - 1; i >= 0; i--) {
    const [sec, name] = units[i]
    if (s >= sec) { const v = Math.floor(s / sec); label = `${v} ${name}${v > 1 ? 's' : ''} ago`; break }
  }
  return label
}

export function duration(sec: number): string {
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

// Friendly comparisons so sizes mean something to anyone.
export function sizeFeel(n: number): string {
  const gb = n / 1024 ** 3
  if (gb >= 40) return `about ${Math.round(gb / 50)} big games`
  if (gb >= 3) return `about ${Math.round(gb / 4)} HD movie${Math.round(gb / 4) === 1 ? '' : 's'}`
  if (gb >= 0.5) return `about ${Math.round((n / 1024 ** 2) / 4)} photos`
  if (n >= 30 * 1024 ** 2) return `about ${Math.max(1, Math.round((n / 1024 ** 2) / 4))} songs`
  return ''
}

export function cpuMood(p: number) {
  if (p < 25) return { label: 'Relaxed', tone: 'good', line: 'Your processor is taking it easy.' }
  if (p < 60) return { label: 'Busy', tone: 'good', line: 'Working steadily — totally normal.' }
  if (p < 85) return { label: 'Working hard', tone: 'warn', line: 'Something is keeping the processor busy.' }
  return { label: 'Maxed out', tone: 'bad', line: 'Your PC may feel slow. Check the busiest apps.' }
}

export function memMood(p: number) {
  if (p < 60) return { label: 'Plenty free', tone: 'good', line: 'Lots of room for more apps.' }
  if (p < 80) return { label: 'Comfortable', tone: 'good', line: 'Memory use is normal.' }
  if (p < 92) return { label: 'Getting full', tone: 'warn', line: 'Closing a few apps would help.' }
  return { label: 'Almost full', tone: 'bad', line: 'Your PC may slow down. Close apps you aren\'t using.' }
}

export function diskMood(p: number) {
  if (p < 75) return { label: 'Roomy', tone: 'good', line: 'You have lots of free space.' }
  if (p < 90) return { label: 'Filling up', tone: 'warn', line: 'A quick clean-up would give you breathing room.' }
  return { label: 'Nearly full', tone: 'bad', line: 'Windows needs free space to run well. Let\'s clean up.' }
}

export const categoryMeta: Record<string, { label: string; color: string }> = {
  folder: { label: 'Folders', color: 'var(--c-folder)' },
  video: { label: 'Videos', color: 'var(--c-video)' },
  audio: { label: 'Music & audio', color: 'var(--c-audio)' },
  image: { label: 'Pictures', color: 'var(--c-image)' },
  archive: { label: 'Zip archives', color: 'var(--c-archive)' },
  installer: { label: 'Installers & disc images', color: 'var(--c-installer)' },
  document: { label: 'Documents', color: 'var(--c-document)' },
  app: { label: 'Apps & game data', color: 'var(--c-app)' },
  disk: { label: 'Virtual disks & backups', color: 'var(--c-disk)' },
  model: { label: 'AI models', color: 'var(--c-model)' },
  code: { label: 'Developer files', color: 'var(--c-code)' },
  other: { label: 'Other files', color: 'var(--c-other)' },
}

export function greeting(): string {
  const h = new Date().getHours()
  if (h < 5) return 'Up late'
  if (h < 12) return 'Good morning'
  if (h < 18) return 'Good afternoon'
  return 'Good evening'
}
