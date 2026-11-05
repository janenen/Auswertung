<script lang="ts">
  // Fullscreen live view of all cameras for a second screen, tiled like the
  // visualisation board. Clicking a tile shows only that camera, clicking
  // again returns to all tiles. The streams are relayed by the server
  // (same origin), so a later calibration can warp them in a canvas.
  import { onDestroy } from 'svelte'
  import { api, cameraStreamURL, type CameraView } from '../lib/api'
  import { t, tx } from '../lib/i18n.svelte'

  const POLL_MS = 2000
  const RETRY_MS = 3000
  const HEADER = 56
  const GAP = 8
  const ASPECT = 3 / 4 // height / width of the tiles (OV2640 is 4:3)

  let cameras = $state<CameraView[]>([])
  let loaded = $state(false)
  let error = $state('')
  let focused = $state('')
  // Incremented to reconnect a stream after an error.
  let attempts = $state<Record<string, number>>({})
  let width = $state(window.innerWidth)
  let height = $state(window.innerHeight)

  async function poll() {
    try {
      cameras = await api.cameras()
      loaded = true
      error = ''
    } catch (e) {
      error = (e as Error).message
    }
  }
  poll()
  const pollTimer = setInterval(poll, POLL_MS)
  const retryTimers = new Set<ReturnType<typeof setTimeout>>()
  onDestroy(() => {
    clearInterval(pollTimer)
    retryTimers.forEach(clearTimeout)
  })

  function reconnect(id: string) {
    const timer = setTimeout(() => {
      retryTimers.delete(timer)
      attempts[id] = (attempts[id] ?? 0) + 1
    }, RETRY_MS)
    retryTimers.add(timer)
  }

  const shown = $derived(focused && cameras.some((c) => c.id === focused) ? [focused] : cameras.map((c) => c.id))

  // Pick the number of rows that makes the tiles as large as possible.
  const layout = $derived.by(() => {
    const k = shown.length
    const w = width - 2 * GAP
    const h = height - HEADER - GAP
    let best = { cols: 1, size: 0 }
    for (let rows = 1; rows <= Math.max(k, 1); rows++) {
      const cols = Math.ceil(k / rows)
      const size = Math.min((w - (cols - 1) * GAP) / cols, (h - (rows - 1) * GAP) / rows / ASPECT)
      if (size > best.size) best = { cols, size }
    }
    return best
  })

  function statusText(c: CameraView) {
    const s = c.status
    if (!s.active) return t('Verbinde…')
    if (!s.connected) return t('Keine Verbindung')
    return `${s.width}×${s.height} · ${t('{fps} Bilder/s', { fps: s.fps.toFixed(1) })}`
  }
</script>

<svelte:window bind:innerWidth={width} bind:innerHeight={height} />

<div class="cameras">
  <header style="height: {HEADER}px">
    <h1>{t('Kameras')}</h1>
    <span class="spacer"></span>
    {#if error}<span class="error">{tx(error)}</span>{/if}
    <a href="#/" class="back">{t('Zur Auswertung')}</a>
  </header>

  {#if loaded && cameras.length === 0}
    <p class="empty">{t('Keine Kameras eingerichtet. Kameras werden unter Stammdaten → Kameras hinzugefügt.')}</p>
  {:else}
    {@const s = layout.size}
    <div class="grid" style="grid-template-columns: repeat({layout.cols}, {s}px); gap: {GAP}px">
      {#each cameras as c (c.id)}
        {@const offline = c.status.active && !c.status.connected}
        <button
          class="tile"
          class:hidden={!shown.includes(c.id)}
          class:zoomed={focused === c.id}
          style="width: {s}px; height: {s * ASPECT}px"
          title={focused ? t('Alle Kameras zeigen') : t('Nur diese Kamera zeigen')}
          onclick={() => (focused = focused ? '' : c.id)}
        >
          {#key `${c.url}#${attempts[c.id] ?? 0}`}
            <img
              src={`${cameraStreamURL(c.id)}?n=${attempts[c.id] ?? 0}`}
              alt={c.name}
              class:offline
              onerror={() => reconnect(c.id)}
            />
          {/key}
          <span class="label" style="font-size: {Math.max(s / 40, 12)}px">
            <span class="dot" class:connected={c.status.connected} class:offline></span>
            <strong>{c.name}</strong>
            <span class="status" title={c.status.error ? tx(c.status.error) : undefined}>{statusText(c)}</span>
          </span>
          {#if offline}
            <span class="overlay" style="font-size: {Math.max(s / 25, 14)}px"
              >{t('Keine Verbindung')}{#if c.status.error}<small>{tx(c.status.error)}</small>{/if}</span
            >
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .cameras {
    position: fixed;
    inset: 0;
    background: #000;
    color: #eee;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1.5rem;
    padding: 0 1rem;
    flex-shrink: 0;
  }
  h1 {
    margin: 0;
    font-size: 1.5rem;
  }
  .back {
    color: #aaa;
    font-size: 0.85rem;
  }
  .grid {
    display: grid;
    justify-content: center;
    align-content: start;
    padding: 0 8px 8px;
  }
  .tile {
    position: relative;
    padding: 0;
    border: 1px solid #333;
    border-radius: 6px;
    overflow: hidden;
    background: #111;
    cursor: zoom-in;
  }
  .tile.zoomed {
    cursor: zoom-out;
  }
  .tile.hidden {
    display: none;
  }
  img {
    width: 100%;
    height: 100%;
    object-fit: contain;
    display: block;
  }
  img.offline {
    opacity: 0.25;
  }
  .label {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 0.6em;
    padding: 0.35em 0.7em;
    background: linear-gradient(transparent, rgb(0 0 0 / 0.75));
    color: #fff;
    text-align: left;
  }
  .status {
    color: #bbb;
    margin-left: auto;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .dot {
    width: 0.7em;
    height: 0.7em;
    border-radius: 50%;
    background: #888;
    flex-shrink: 0;
  }
  .dot.connected {
    background: #3c3;
  }
  .dot.offline {
    background: #d33;
  }
  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.3em;
    color: #fff;
    font-weight: 600;
  }
  .overlay small {
    font-weight: 400;
    font-size: 0.6em;
    color: #ccc;
  }
  .empty {
    text-align: center;
    color: #aaa;
    font-size: 1.25rem;
    margin-top: 20vh;
    padding: 0 1rem;
  }
</style>
