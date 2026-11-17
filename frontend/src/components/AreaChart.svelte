<script lang="ts">
  // Responsive live area chart with up to two series and a hover readout.
  let { series, max = undefined, format = (v: number) => v.toFixed(0), height = 180, points = 120 }: {
    series: { label: string; color: string; data: number[] }[]
    max?: number
    format?: (v: number) => string
    height?: number
    points?: number
  } = $props()

  let width = $state(600)
  let hover = $state<number | null>(null)
  const pad = { t: 10, b: 20, l: 0, r: 0 }

  const hi = $derived(max ?? Math.max(1, ...series.flatMap((s) => s.data)) * 1.2)
  const step = $derived(width / (points - 1))

  function pathFor(data: number[], area: boolean) {
    if (data.length < 2) return ''
    const off = points - data.length
    const h = height - pad.t - pad.b
    const pts = data.map((v, i) => [(i + off) * step, pad.t + h - (Math.min(v, hi) / hi) * h])
    let d = `M${pts[0][0]},${pts[0][1]}`
    for (let i = 1; i < pts.length; i++) {
      const [x0, y0] = pts[i - 1], [x1, y1] = pts[i]
      const mx = (x0 + x1) / 2
      d += ` C${mx},${y0} ${mx},${y1} ${x1},${y1}`
    }
    return area ? `${d} L${pts[pts.length - 1][0]},${height - pad.b} L${pts[0][0]},${height - pad.b} Z` : d
  }

  function onMove(e: PointerEvent) {
    const r = (e.currentTarget as SVGElement).getBoundingClientRect()
    hover = Math.round((e.clientX - r.left) / step)
  }

  const ids = [0, 1, 2].map(() => `ac${Math.random().toString(36).slice(2, 8)}`)
  const len = $derived(Math.max(...series.map((s) => s.data.length)))
</script>

<div class="chart" bind:clientWidth={width}>
  <svg {width} {height} onpointermove={onMove} onpointerleave={() => (hover = null)} role="img">
    <defs>
      {#each series as s, i}
        <linearGradient id={ids[i]} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stop-color={s.color} stop-opacity=".30" />
          <stop offset="1" stop-color={s.color} stop-opacity="0" />
        </linearGradient>
      {/each}
    </defs>
    {#each [0.25, 0.5, 0.75, 1] as g}
      <line x1="0" x2={width} y1={pad.t + (height - pad.t - pad.b) * (1 - g)} y2={pad.t + (height - pad.t - pad.b) * (1 - g)} class="grid" />
    {/each}
    <line x1="0" x2={width} y1={height - pad.b} y2={height - pad.b} class="axis" />
    {#each series as s, i}
      <path d={pathFor(s.data, true)} fill="url(#{ids[i]})" />
      <path d={pathFor(s.data, false)} fill="none" stroke={s.color} stroke-width="2" stroke-linejoin="round" />
    {/each}
    {#if hover !== null && hover >= points - len}
      <line x1={hover * step} x2={hover * step} y1={pad.t} y2={height - pad.b} class="cursor" />
      {#each series as s}
        {@const v = s.data[hover - (points - s.data.length)]}
        {#if v !== undefined}
          {@const h = height - pad.t - pad.b}
          <circle cx={hover * step} cy={pad.t + h - (Math.min(v, hi) / hi) * h} r="4" fill={s.color} stroke="var(--surface)" stroke-width="2" />
        {/if}
      {/each}
    {/if}
    <text x="0" y={height - 4} class="lbl">2 min ago</text>
    <text x={width} y={height - 4} class="lbl" text-anchor="end">now</text>
    <text x={width - 2} y={pad.t + 10} class="lbl" text-anchor="end">{format(hi)}</text>
  </svg>
  {#if hover !== null && hover >= points - len}
    <div class="tip" style="left:{Math.min(width - 150, Math.max(0, hover * step - 70))}px">
      {#each series as s}
        {@const v = s.data[hover - (points - s.data.length)]}
        {#if v !== undefined}<div><i style="background:{s.color}"></i>{s.label}<b class="num">{format(v)}</b></div>{/if}
      {/each}
      <small>{Math.max(0, points - 1 - hover)}s ago</small>
    </div>
  {/if}
</div>

<style>
  .chart { position: relative; width: 100%; }
  svg { display: block; overflow: visible; }
  .grid { stroke: var(--border); stroke-dasharray: 2 4; }
  .axis { stroke: var(--border-strong); }
  .cursor { stroke: var(--border-strong); }
  .lbl { fill: var(--faint); font-size: 10.5px; }
  .tip {
    position: absolute; top: -6px; min-width: 140px; pointer-events: none;
    padding: 8px 10px; border-radius: 10px; background: var(--surface-2); box-shadow: var(--shadow-pop); font-size: 12px;
  }
  .tip div { display: flex; align-items: center; gap: 6px; }
  .tip i { width: 8px; height: 8px; border-radius: 3px; }
  .tip b { margin-left: auto; font-weight: 600; }
  .tip small { color: var(--faint); }
</style>
