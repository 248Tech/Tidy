<script lang="ts">
  import type { TreeItem } from '../lib/api'
  import { bytes, categoryMeta } from '../lib/format'
  import { Check } from '@lucide/svelte'

  let { items, total, selected, onopen, ontoggle }: {
    items: TreeItem[]
    total: number
    selected: Set<string>
    onopen: (it: TreeItem) => void
    ontoggle: (it: TreeItem) => void
  } = $props()

  let w = $state(800)
  let h = $state(440)
  let hover = $state<{ it: TreeItem; x: number; y: number } | null>(null)

  type Rect = { it: TreeItem; x: number; y: number; w: number; h: number }

  function worst(row: number[], side: number) {
    const s = row.reduce((a, b) => a + b, 0)
    const mx = Math.max(...row), mn = Math.min(...row)
    return Math.max((side * side * mx) / (s * s), (s * s) / (side * side * mn))
  }

  const rects = $derived.by(() => {
    const sum = items.reduce((a, b) => a + b.size, 0)
    if (!sum || w < 10 || h < 10) return [] as Rect[]
    const min = sum * 0.0025
    let shown = items.filter((i) => i.size >= min && i.size > 0)
    const rest = sum - shown.reduce((a, b) => a + b.size, 0)
    if (rest > 0) shown = [...shown, { name: 'Small items', path: '', size: rest, isDir: false, category: 'other', other: true }]
    shown.sort((a, b) => b.size - a.size)

    const out: Rect[] = []
    let x = 0, y = 0, cw = w, ch = h
    const scale = (w * h) / sum
    const queue = shown.map((it) => ({ it, a: it.size * scale }))
    let row: typeof queue = []
    const place = () => {
      const s = row.reduce((a, b) => a + b.a, 0)
      if (cw >= ch) {
        const colW = s / ch
        let yy = y
        for (const r of row) { const rh = r.a / colW; out.push({ it: r.it, x, y: yy, w: colW, h: rh }); yy += rh }
        x += colW; cw -= colW
      } else {
        const rowH = s / cw
        let xx = x
        for (const r of row) { const rw = r.a / rowH; out.push({ it: r.it, x: xx, y, w: rw, h: rowH }); xx += rw }
        y += rowH; ch -= rowH
      }
      row = []
    }
    while (queue.length) {
      const side = Math.min(cw, ch)
      const next = queue[0]
      if (!row.length || worst([...row.map((r) => r.a), next.a], side) <= worst(row.map((r) => r.a), side)) {
        row.push(next)
        queue.shift()
      } else place()
    }
    if (row.length) place()
    return out
  })

  function move(e: PointerEvent, it: TreeItem) {
    const box = (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect()
    hover = { it, x: e.clientX - box.left, y: e.clientY - box.top }
  }
</script>

<div class="map" bind:clientWidth={w} bind:clientHeight={h} onpointerleave={() => (hover = null)}>
  {#each rects as r (r.it.path || r.it.name)}
    {@const color = categoryMeta[r.it.category]?.color ?? 'var(--c-other)'}
    {@const big = r.w > 90 && r.h > 46}
    <button
      class="cell"
      class:dir={r.it.isDir}
      class:other={r.it.other}
      class:sel={selected.has(r.it.path)}
      style="left:{r.x}px;top:{r.y}px;width:{r.w}px;height:{r.h}px;--c:{color}"
      onpointermove={(e) => move(e, r.it)}
      onclick={() => !r.it.other && (r.it.isDir ? onopen(r.it) : ontoggle(r.it))}
      aria-label="{r.it.name}, {bytes(r.it.size)}"
      tabindex={r.it.other ? -1 : 0}
    >
      <span class="fill"></span>
      {#if big}
        <span class="lbl">
          <strong class="ellipsis">{r.it.name}</strong>
          <span class="num">{bytes(r.it.size)}</span>
        </span>
      {/if}
      {#if selected.has(r.it.path)}<span class="tick"><Check size={12} strokeWidth={3} /></span>{/if}
    </button>
  {/each}

  {#if hover}
    <div class="tip" style="left:{Math.min(hover.x + 14, w - 230)}px;top:{Math.min(hover.y + 14, h - 70)}px">
      <strong class="ellipsis">{hover.it.name}</strong>
      <span><b class="num">{bytes(hover.it.size)}</b> · {((hover.it.size / Math.max(1, total)) * 100).toFixed(1)}% · {hover.it.isDir ? 'Folder — click to open' : hover.it.other ? 'Many small files' : `${categoryMeta[hover.it.category]?.label ?? 'File'} — click to select`}</span>
    </div>
  {/if}
</div>

<style>
  .map { position: relative; width: 100%; height: 100%; min-height: 300px; }
  .cell {
    position: absolute; padding: 0; border: 0; background: none;
    text-align: left; overflow: hidden; color: #fff;
    transition: left .45s var(--ease), top .45s var(--ease), width .45s var(--ease), height .45s var(--ease);
  }
  .fill {
    position: absolute; inset: 1.5px; border-radius: 9px;
    background: linear-gradient(145deg, color-mix(in srgb, var(--c) 92%, white 8%), color-mix(in srgb, var(--c) 70%, black 30%));
    opacity: .82; transition: opacity .15s, transform .2s var(--ease), box-shadow .15s;
  }
  .cell.dir .fill { opacity: .72; }
  .cell.other .fill { opacity: .35; background: repeating-linear-gradient(45deg, var(--c-other), var(--c-other) 6px, transparent 6px, transparent 12px); }
  .cell:hover .fill { opacity: 1; box-shadow: 0 0 0 2px rgba(255, 255, 255, .7) inset; }
  .cell.sel .fill { opacity: 1; box-shadow: 0 0 0 3px #fff inset, 0 0 0 5px var(--c) inset; }
  .cell:focus-visible { outline: none; }
  .cell:focus-visible .fill { box-shadow: 0 0 0 2px #fff inset; }
  .lbl { position: absolute; left: 10px; right: 10px; top: 8px; display: flex; flex-direction: column; line-height: 1.3; text-shadow: 0 1px 2px rgba(0, 0, 0, .35); pointer-events: none; }
  .lbl strong { font-size: 12.5px; font-weight: 650; }
  .lbl span { font-size: 11.5px; opacity: .9; }
  .tick { position: absolute; right: 8px; top: 8px; width: 20px; height: 20px; border-radius: 6px; background: #fff; color: #111; display: grid; place-items: center; }
  .tip {
    position: absolute; z-index: 3; pointer-events: none; max-width: 300px;
    display: flex; flex-direction: column; gap: 2px; padding: 9px 12px; border-radius: 10px;
    background: var(--surface-2); box-shadow: var(--shadow-pop); font-size: 12px; color: var(--muted);
  }
  .tip strong { color: var(--text); font-size: 13px; }
  .tip b { color: var(--text); }
</style>
