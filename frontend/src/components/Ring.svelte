<script lang="ts">
  import { Tween } from 'svelte/motion'
  import { cubicOut } from 'svelte/easing'
  import type { Snippet } from 'svelte'

  let { value = 0, size = 120, stroke = 10, tone = 'accent', children }: {
    value: number; size?: number; stroke?: number; tone?: string; children?: Snippet
  } = $props()

  const t = new Tween(0, { duration: 900, easing: cubicOut })
  $effect(() => { t.target = Math.max(0, Math.min(100, value || 0)) })

  const r = $derived((size - stroke) / 2)
  const c = $derived(2 * Math.PI * r)
  const gid = `rg${Math.random().toString(36).slice(2, 8)}`
  const stops: Record<string, [string, string]> = {
    accent: ['#8b7bff', '#3dd6c4'],
    good: ['#34d399', '#3dd6c4'],
    warn: ['#fbbf24', '#fb923c'],
    bad: ['#fb7185', '#f43f5e'],
  }
  const s = $derived(stops[tone] ?? stops.accent)
</script>

<div class="ring" style="width:{size}px;height:{size}px">
  <svg width={size} height={size} viewBox="0 0 {size} {size}">
    <defs>
      <linearGradient id={gid} x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stop-color={s[0]} />
        <stop offset="1" stop-color={s[1]} />
      </linearGradient>
    </defs>
    <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--surface-3)" stroke-width={stroke} />
    <circle
      cx={size / 2} cy={size / 2} r={r} fill="none"
      stroke="url(#{gid})" stroke-width={stroke} stroke-linecap="round"
      stroke-dasharray={c} stroke-dashoffset={c * (1 - t.current / 100)}
      transform="rotate(-90 {size / 2} {size / 2})"
      style="filter: drop-shadow(0 0 {stroke * 0.8}px {s[0]}55)"
    />
  </svg>
  <div class="center">{@render children?.()}</div>
</div>

<style>
  .ring { position: relative; flex: none; }
  svg { display: block; }
  .center { position: absolute; inset: 0; display: grid; place-items: center; text-align: center; }
</style>
