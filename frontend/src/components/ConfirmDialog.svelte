<script lang="ts">
  import { fade, scale } from 'svelte/transition'
  import { Trash2, Power, ShieldAlert, Sparkles, TriangleAlert } from '@lucide/svelte'
  import { ui } from '../lib/ui.svelte'

  const icons = { trash: Trash2, power: Power, shield: ShieldAlert, sparkles: Sparkles, alert: TriangleAlert }
  let confirmBtn = $state<HTMLButtonElement>()

  $effect(() => {
    if (ui.confirm) setTimeout(() => confirmBtn?.focus(), 50)
  })

  function key(e: KeyboardEvent) {
    if (!ui.confirm) return
    if (e.key === 'Escape') ui.confirm.resolve(false)
  }
</script>

<svelte:window onkeydown={key} />

{#if ui.confirm}
  {@const c = ui.confirm}
  {@const Icon = icons[c.icon ?? 'alert']}
  <div class="scrim" transition:fade={{ duration: 150 }} onclick={() => c.resolve(false)} role="presentation"></div>
  <div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="cd-title" transition:scale={{ start: 0.94, duration: 220 }}>
    <div class="icon {c.tone ?? 'primary'}"><Icon size={22} /></div>
    <h2 id="cd-title">{c.title}</h2>
    <p class="body">{c.body}</p>
    {#if c.items?.length}
      <ul class="items">
        {#each c.items.slice(0, 6) as it}
          <li><span class="ellipsis">{it.label}</span>{#if it.sub}<span class="sub num">{it.sub}</span>{/if}</li>
        {/each}
        {#if c.items.length > 6}<li class="more">and {c.items.length - 6} more…</li>{/if}
      </ul>
    {/if}
    {#if c.note}<p class="note">{c.note}</p>{/if}
    <div class="actions">
      <button class="btn" onclick={() => c.resolve(false)}>Cancel</button>
      <button bind:this={confirmBtn} class="btn {c.tone === 'danger' ? 'danger' : 'primary'}" onclick={() => c.resolve(true)}>{c.confirmLabel}</button>
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 90; background: rgba(5, 6, 10, .55); backdrop-filter: blur(4px); }
  .dialog {
    position: fixed; z-index: 91; left: 50%; top: 50%; translate: -50% -50%;
    width: min(440px, calc(100vw - 32px)); padding: 26px;
    border-radius: var(--radius-lg); background: var(--surface); box-shadow: var(--shadow-pop);
  }
  .icon { width: 46px; height: 46px; border-radius: 14px; display: grid; place-items: center; margin-bottom: 16px; }
  .icon.danger { background: var(--bad-soft); color: var(--bad); }
  .icon.primary { background: var(--accent-soft); color: var(--accent); }
  h2 { font-size: 18px; margin-bottom: 6px; }
  .body { color: var(--muted); }
  .items { list-style: none; margin: 16px 0 0; padding: 6px; border-radius: 12px; background: var(--surface-2); max-height: 220px; overflow: auto; }
  .items li { display: flex; justify-content: space-between; gap: 12px; padding: 7px 10px; font-size: 13px; border-radius: 8px; }
  .items li + li { border-top: 1px solid var(--border); }
  .items .sub { color: var(--muted); flex: none; }
  .items .more { color: var(--faint); }
  .note { margin-top: 14px; font-size: 12.5px; color: var(--muted); padding: 10px 12px; border-radius: 10px; background: var(--warn-soft); }
  .actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 22px; }
</style>
