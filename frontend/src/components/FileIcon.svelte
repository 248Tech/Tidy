<script lang="ts">
  import { Folder, Film, Music, Image, Archive, Package, FileText, Gamepad2, Database, Code, File, Brain } from '@lucide/svelte'
  import { categoryMeta } from '../lib/format'
  let { category = 'other', size = 32, dir = false }: { category?: string; size?: number; dir?: boolean } = $props()
  const icons: Record<string, any> = { folder: Folder, video: Film, audio: Music, image: Image, archive: Archive, installer: Package, document: FileText, app: Gamepad2, disk: Database, model: Brain, code: Code, other: File }
  const Icon = $derived(dir ? Folder : icons[category] ?? File)
</script>

<span class="fi" style="width:{size}px;height:{size}px;--c:{categoryMeta[category]?.color ?? 'var(--c-other)'}">
  <Icon size={size * 0.5} />
</span>

<style>
  .fi { flex: none; display: grid; place-items: center; border-radius: 9px; color: var(--c); background: color-mix(in srgb, var(--c) 16%, transparent); }
</style>
