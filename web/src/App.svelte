<script lang="ts">
  import type { Component } from 'svelte'
  import { api } from './lib/api'
  import { meta } from './lib/meta.svelte'
  import { route, matchPath, href } from './lib/router.svelte'
  import Confirm from './lib/Confirm.svelte'
  import Home from './routes/Home.svelte'
  import CompetitionForm from './routes/CompetitionForm.svelte'
  import CompetitionPage from './routes/Competition.svelte'
  import Entry from './routes/Entry.svelte'
  import Match from './routes/Match.svelte'
  import Results from './routes/Results.svelte'
  import Archive from './routes/Archive.svelte'
  import People from './routes/People.svelte'
  import Board from './routes/Board.svelte'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const routes: [string, Component<any>][] = [
    ['/', Home],
    ['/competitions/new', CompetitionForm],
    ['/quick', CompetitionForm],
    ['/competitions/:id', CompetitionPage],
    ['/competitions/:id/entry', Entry],
    ['/competitions/:id/results', Results],
    ['/entry', Entry],
    ['/matches/:id', Match],
    ['/archive', Archive],
    ['/people', People],
  ]

  const current = $derived.by(() => {
    for (const [pattern, component] of routes) {
      const params = matchPath(pattern, route.path)
      if (params) return { component, params, pattern }
    }
    return null
  })

  let loaded = $state(false)
  let loadError = $state('')
  api
    .meta()
    .then((m) => {
      Object.assign(meta, m)
      loaded = true
    })
    .catch((e) => (loadError = e.message))

  let confirmQuit = $state(false)
  let stopped = $state(false)

  async function quit() {
    try {
      await api.shutdown()
    } finally {
      stopped = true
    }
  }

  const nav = [
    { path: '/', label: 'Wettkämpfe' },
    { path: '/quick', label: 'Schnellauswertung' },
    { path: '/archive', label: 'Archiv' },
    { path: '/people', label: 'Stammdaten' },
  ]
  const section = $derived(
    route.path === '/' || route.path.startsWith('/competitions')
      ? '/'
      : route.path === '/entry'
        ? '/quick'
        : route.path,
  )
</script>

{#if route.path === '/board'}
  {#if loaded}<Board query={route.query} />{/if}
{:else if stopped}
  <main class="stopped">
    <h1>Auswertung beendet</h1>
    <p class="muted">Das Programm wurde beendet. Dieses Fenster kann geschlossen werden.</p>
  </main>
{:else}
  <header>
    <a class="brand" href="#/">Auswertung</a>
    <nav>
      {#each nav as item}
        <a href={href(item.path)} class:current={section === item.path}>{item.label}</a>
      {/each}
    </nav>
    <span class="spacer"></span>
    <a class="button" href={href('/board')} target="_blank" title="Visualisierungsfenster öffnen, z.B. für einen zweiten Bildschirm"
      >Visualisierung</a
    >
    <button onclick={() => (confirmQuit = true)}>Programm beenden</button>
  </header>

  <main>
    {#if loadError}
      <p class="error">Server nicht erreichbar: {loadError}</p>
    {:else if !loaded}
      <p class="muted">Lade…</p>
    {:else if current}
      {#key route.path + JSON.stringify(route.query)}
        <current.component {...current.params} query={route.query} quick={current.pattern === '/quick' || current.pattern === '/entry'} />
      {/key}
    {:else}
      <p>Seite nicht gefunden. <a href="#/">Zur Übersicht</a></p>
    {/if}
  </main>

  <Confirm
    bind:open={confirmQuit}
    title="Wirklich beenden?"
    message="Der lokale Server wird gestoppt. Laufende Einlesevorgänge werden abgebrochen."
    confirmLabel="Beenden"
    danger
    onconfirm={quit}
  />
{/if}

<style>
  header {
    display: flex;
    align-items: center;
    gap: 1.25rem;
    padding: 0.6rem 1.25rem;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    z-index: 10;
    flex-wrap: wrap;
  }
  .brand {
    font-weight: 700;
    font-size: 1.1rem;
    color: var(--text);
    text-decoration: none;
  }
  nav {
    display: flex;
    gap: 0.25rem;
    flex-wrap: wrap;
  }
  nav a {
    padding: 0.35rem 0.7rem;
    border-radius: 6px;
    color: var(--muted);
    text-decoration: none;
  }
  nav a:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  nav a.current {
    background: var(--surface-2);
    color: var(--text);
    font-weight: 600;
  }
  main {
    max-width: 72rem;
    margin: 0 auto;
    padding: 1.5rem 1.25rem 3rem;
  }
  .stopped {
    text-align: center;
    padding-top: 20vh;
  }
</style>
