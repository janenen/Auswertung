<script lang="ts">
  import { onDestroy } from 'svelte'
  import { api, type ReadingEvent, type Shot } from '../../lib/api'
  import Target from '../../lib/Target.svelte'

  let {
    request,
    targetType,
    total,
    already,
    onfinished,
    onback,
  }: {
    request: Parameters<typeof api.startReading>[0]
    targetType: string
    total: number
    already: number
    onfinished: (matchId: string) => void
    onback: () => void
  } = $props()

  let sessionId = $state('')
  let shots = $state<Shot[]>([])
  let log = $state<{ type: string; message: string }[]>([])
  let done = $state(false)
  let stopping = $state(false)
  let source: EventSource | undefined

  async function start() {
    log.push({ type: 'status', message: 'Einstellungen werden übergeben…' })
    try {
      const { id } = await api.startReading(request)
      sessionId = id
    } catch (e) {
      log.push({ type: 'error', message: (e as Error).message })
      done = true
      return
    }
    source = new EventSource(`/api/readings/${sessionId}/events`)
    source.onmessage = (msg) => {
      const e: ReadingEvent = JSON.parse(msg.data)
      if (e.type === 'shot' && e.shot) shots.push(e.shot)
      else if (e.type === 'done') {
        done = true
        source?.close()
        if (e.matchId) onfinished(e.matchId)
      } else if (e.message) log.push({ type: e.type, message: e.message })
    }
  }
  start()

  async function stop(keep: boolean) {
    stopping = true
    try {
      await api.stopReading(sessionId, keep)
    } catch (e) {
      log.push({ type: 'error', message: (e as Error).message })
    }
  }

  onDestroy(() => {
    source?.close()
    // Leaving the page cancels a running reading, like "Zurück" did before.
    if (sessionId && !done) api.stopReading(sessionId, false).catch(() => {})
  })

  const last = $derived(shots.at(-1))
</script>

<div class="layout">
  <section class="card target">
    <Target type={targetType} {shots} />
  </section>

  <section class="card side">
    <div class="progress">
      <span class="big">{already + shots.length}</span>
      <span class="muted">/ {total} Schuss</span>
    </div>
    <progress max={total} value={already + shots.length}></progress>
    {#if last}
      <p class="last">
        Letzter Schuss: <strong>{last.ringe.toFixed(1)}</strong>
        <span class="muted">Teiler {last.teiler.toFixed(1)}</span>
      </p>
    {/if}

    <ul class="log">
      {#each log as l}
        <li class:error={l.type === 'error'}>{l.message}</li>
      {/each}
    </ul>

    <div class="shots">
      {#each shots as s, i}
        <span title="Teiler {s.teiler.toFixed(1)}"><small class="muted">{already + i + 1}</small> {s.ringe.toFixed(1)}</span>
      {/each}
    </div>

    <div class="actions end">
      {#if done}
        <button onclick={onback}>Zurück</button>
      {:else}
        <button class="danger" disabled={!sessionId || stopping} onclick={() => stop(false)}>Abbrechen</button>
        <button class="primary" disabled={!sessionId || stopping} onclick={() => stop(true)}
          >Stopp &amp; speichern</button
        >
      {/if}
    </div>
  </section>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: minmax(16rem, 30rem) 1fr;
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 720px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  .target {
    padding: 0.5rem;
  }
  .side {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .progress .big {
    font-size: 2.2rem;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }
  progress {
    width: 100%;
  }
  .last {
    margin: 0;
  }
  .log {
    list-style: none;
    padding: 0;
    margin: 0;
    font-size: 0.9rem;
  }
  .log li::before {
    content: '› ';
    color: var(--muted);
  }
  .shots {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(4.2rem, 1fr));
    gap: 0.25rem 0.75rem;
    font-variant-numeric: tabular-nums;
  }
  .end {
    justify-content: flex-end;
    margin-top: 0.5rem;
  }
</style>
