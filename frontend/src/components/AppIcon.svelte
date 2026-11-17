<script lang="ts">
  import { iconUrl } from '../lib/api'
  let { exe = '', name = '', size = 32 }: { exe?: string; name?: string; size?: number } = $props()

  let failed = $state(false)
  $effect(() => { exe; failed = false })

  const hues = [258, 200, 168, 330, 28, 142, 290, 214]
  const hue = $derived(hues[[...(name || '?')].reduce((a, c) => a + c.charCodeAt(0), 0) % hues.length])
</script>

<div class="icon" style="width:{size}px;height:{size}px;border-radius:{size * 0.28}px">
  {#if exe && !failed}
    <img src={iconUrl(exe)} alt="" width={size * 0.82} height={size * 0.82} onerror={() => (failed = true)} loading="lazy" draggable="false" />
  {:else}
    <span style="background:linear-gradient(135deg,hsl({hue} 70% 62%),hsl({hue + 40} 70% 52%));font-size:{size * 0.42}px">{(name || '?').trim()[0]?.toUpperCase()}</span>
  {/if}
</div>

<style>
  .icon { flex: none; display: grid; place-items: center; overflow: hidden; background: var(--surface-2); border: 1px solid var(--border); }
  img { object-fit: contain; image-rendering: -webkit-optimize-contrast; }
  span { width: 100%; height: 100%; display: grid; place-items: center; color: #fff; font-weight: 700; }
</style>
