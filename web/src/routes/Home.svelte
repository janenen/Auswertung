<script lang="ts">
  import { api, type CompetitionSummary } from '../lib/api'
  import { href } from '../lib/router.svelte'

  let competitions = $state<CompetitionSummary[] | null>(null)
  let error = $state('')
  api
    .competitions(true)
    .then((c) => (competitions = c))
    .catch((e) => (error = e.message))
</script>

<div class="actions head">
  <h1>Laufende Wettkämpfe</h1>
  <span class="spacer"></span>
  <a class="button" href={href('/quick')}>Schnellauswertung</a>
  <a class="button primary" href={href('/competitions/new')}>+ Wettkampf hinzufügen</a>
</div>

{#if error}
  <p class="error">{error}</p>
{:else if competitions === null}
  <p class="muted">Lade…</p>
{:else if competitions.length === 0}
  <div class="card empty">
    <p>Kein laufender Wettkampf.</p>
    <p class="muted">
      Lege einen neuen Wettkampf an, lade einen aus dem <a href={href('/archive')}>Archiv</a> oder starte eine
      Schnellauswertung, die keinem Wettkampf zugeordnet wird.
    </p>
  </div>
{:else}
  <div class="grid">
    {#each competitions as c (c.id)}
      <a class="card competition" href={href(`/competitions/${c.id}`)}>
        <strong>{c.name}</strong>
        <span class="muted">{c.date} · {c.type_of_target} · {c.count} Schuss</span>
        <span class="muted">{c.modus}</span>
        <span class="count">{c.entryCount} {c.entryCount === 1 ? 'Beitrag' : 'Beiträge'}</span>
      </a>
    {/each}
  </div>
{/if}

<style>
  .head {
    margin-bottom: 1rem;
  }
  .head h1 {
    margin: 0;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
    gap: 1rem;
  }
  .competition {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    color: inherit;
    text-decoration: none;
    transition: border-color 0.15s;
  }
  .competition:hover {
    border-color: var(--accent);
  }
  .competition strong {
    font-size: 1.1rem;
  }
  .count {
    margin-top: 0.5rem;
    font-weight: 600;
  }
  .empty p:first-child {
    margin-top: 0;
    font-weight: 600;
  }
</style>
