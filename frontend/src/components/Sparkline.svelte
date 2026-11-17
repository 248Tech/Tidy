<script lang="ts">
  let { data = [], width = 120, height = 32, color = 'var(--accent)', max = undefined, fill = true }: {
    data: number[]; width?: number; height?: number; color?: string; max?: number; fill?: boolean
  } = $props()

  const gid = `sp${Math.random().toString(36).slice(2, 8)}`

  const path = $derived.by(() => {
    if (data.length < 2) return { line: '', area: '' }
    const hi = max ?? Math.max(...data) * 1.15
    const lo = max !== undefined ? 0 : Math.min(...data) * 0.85
    const span = hi - lo || 1
    const step = width / (data.length - 1)
    const pts = data.map((v, i) => [i * step, height - 2 - ((v - lo) / span) * (height - 4)])
    // smooth curve (Catmull-Rom → Bezier)
    let d = `M${pts[0][0]},${pts[0][1]}`
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[i - 1] ?? pts[i], p1 = pts[i], p2 = pts[i + 1], p3 = pts[i + 2] ?? p2
      const c1 = [p1[0] + (p2[0] - p0[0]) / 6, p1[1] + (p2[1] - p0[1]) / 6]
      const c2 = [p2[0] - (p3[0] - p1[0]) / 6, p2[1] - (p3[1] - p1[1]) / 6]
      d += ` C${c1[0]},${c1[1]} ${c2[0]},${c2[1]} ${p2[0]},${p2[1]}`
    }
    return { line: d, area: `${d} L${width},${height} L0,${height} Z` }
  })
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" aria-hidden="true">
  <defs>
    <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color={color} stop-opacity=".28" />
      <stop offset="1" stop-color={color} stop-opacity="0" />
    </linearGradient>
  </defs>
  {#if fill && path.area}<path d={path.area} fill="url(#{gid})" />{/if}
  {#if path.line}<path d={path.line} fill="none" stroke={color} stroke-width="1.75" stroke-linecap="round" />{/if}
</svg>

<style>
  svg { display: block; overflow: visible; }
</style>
