<script lang="ts">
  // Fullscreen visualisation for a second screen: shows the active
  // competitions one after another, including readings in progress.
  // ?interval=<seconds> sets how long each competition is shown.
  import { onDestroy } from 'svelte'
  import { api, type BoardCompetition } from '../lib/api'
  import Target from '../lib/Target.svelte'

  let { query = {} }: { query?: Record<string, string> } = $props()

  const POLL_MS = 2000
  const interval = $derived(Math.max(Number(query.interval) || 10, 3) * 1000)

  let competitions = $state<BoardCompetition[]>([])
  let currentId = $state('')
  let error = $state('')
  let width = $state(window.innerWidth)
  let height = $state(window.innerHeight)

  async function poll() {
    try {
      competitions = await api.board()
      error = ''
    } catch (e) {
      error = (e as Error).message
    }
  }
  poll()
  const pollTimer = setInterval(poll, POLL_MS)

  $effect(() => {
    const timer = setInterval(() => {
      const i = competitions.findIndex((c) => c.id === currentId)
      currentId = competitions[(i + 1) % Math.max(competitions.length, 1)]?.id ?? ''
    }, interval)
    return () => clearInterval(timer)
  })

  onDestroy(() => clearInterval(pollTimer))

  const current = $derived(competitions.find((c) => c.id === currentId) ?? competitions[0])

  const HEADER = 72
  const INFO = 0.5 // info area height relative to tile width
  const GAP = 12

  // Pick the number of rows that makes the tiles as large as possible.
  const layout = $derived.by(() => {
    const k = current?.tiles.length ?? 0
    const w = width - 2 * GAP
    const h = height - HEADER - GAP
    let best = { cols: 1, size: 0 }
    for (let rows = 1; rows <= Math.max(k, 1); rows++) {
      const cols = Math.ceil(k / rows)
      const size = Math.min((w - (cols - 1) * GAP) / cols, (h - (rows - 1) * GAP) / rows / (1 + INFO))
      if (size > best.size) best = { cols, size }
    }
    return best
  })
</script>

<svelte:window bind:innerWidth={width} bind:innerHeight={height} />

<div class="board">
  <header style="height: {HEADER}px">
    <h1>{current?.name ?? 'Visualisierung'}</h1>
    {#if competitions.length > 1}
      <div class="pages" aria-label="Wettkämpfe">
        {#each competitions as c (c.id)}
          <button class:current={c.id === current?.id} onclick={() => (currentId = c.id)} title={c.name}></button>
        {/each}
      </div>
    {/if}
    <span class="spacer"></span>
    {#if error}<span class="error">{error}</span>{/if}
    <a href="#/" class="back">Zur Auswertung</a>
  </header>

  {#if !current}
    <p class="empty">Keine laufenden Wettkämpfe mit Ergebnissen.</p>
  {:else}
    {@const s = layout.size}
    <div class="grid" style="grid-template-columns: repeat({layout.cols}, {s}px); gap: {GAP}px">
      {#each current.tiles as t (t.key)}
        <article class="tile" style="width: {s}px">
          <Target type={t.targetType} shots={t.shots} zoom numbered />
          <div class="info" style="height: {s * INFO}px; font-size: {s / 16}px">
            <div class="name">
              {#if t.live}<span class="live" title="Wird gerade eingelesen"></span>{/if}
              {t.shooter}
            </div>
            <div class="result" style="font-size: {s / 7}px">{t.result}</div>
            <div class="count">{t.shots.length} / {current.count}</div>
            <div class="series">
              {#each t.series as r}<span>{r}</span>{/each}
            </div>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>

<style>
  .board {
    position: fixed;
    inset: 0;
    background: var(--bg);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1.5rem;
    padding: 0 1.25rem;
    flex-shrink: 0;
  }
  h1 {
    margin: 0;
    font-size: 2rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pages {
    display: flex;
    gap: 0.4rem;
  }
  .pages button {
    width: 12px;
    height: 12px;
    padding: 0;
    border-radius: 50%;
    background: var(--border);
    border: none;
  }
  .pages button.current {
    background: var(--accent);
  }
  .back {
    color: var(--muted);
    font-size: 0.85rem;
  }
  .grid {
    display: grid;
    justify-content: center;
    align-content: start;
    padding: 0 12px 12px;
  }
  .tile {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .info {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.15em;
    font-variant-numeric: tabular-nums;
    border-top: 1px solid var(--border);
  }
  .name {
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 0.4em;
    max-width: 95%;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .result {
    font-weight: 700;
    line-height: 1.05;
  }
  .count {
    color: var(--muted);
  }
  .series {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 0 0.8em;
    color: var(--muted);
  }
  .live {
    width: 0.6em;
    height: 0.6em;
    border-radius: 50%;
    background: var(--shot-10);
    animation: pulse 1.2s ease-in-out infinite;
    flex-shrink: 0;
  }
  @keyframes pulse {
    50% {
      opacity: 0.25;
    }
  }
  .empty {
    text-align: center;
    color: var(--muted);
    font-size: 1.5rem;
    margin-top: 20vh;
  }
</style>
