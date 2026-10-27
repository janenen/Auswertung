<script lang="ts">
  import { api, type CompetitionView } from '../lib/api'
  import { href, navigate } from '../lib/router.svelte'

  let { id }: { id: string } = $props()

  let view = $state<CompetitionView | null>(null)
  let error = $state('')
  $effect(() => {
    api
      .competition(id)
      .then((v) => (view = v))
      .catch((e) => (error = e.message))
  })

  const c = $derived(view?.competition)
  const incomplete = $derived(view?.entries.filter((e) => e.active) ?? [])
</script>

{#if error}
  <p class="error">{error}</p>
{:else if view && c}
  <div class="actions head">
    <div>
      <h1>{c.name}</h1>
      <p class="muted sub">
        {c.date} · Scheibe {c.type_of_target} · {c.count} Schuss · {c.shots_per_target} pro Scheibe · {c.modus}
        {#if !c.active}<span class="badge">beendet</span>{/if}
      </p>
      {#if view.league}
        <p class="muted sub">
          {view.league.league}
          {view.league.group} · {view.league.discipline} · Heim: {view.league.homeClubName} – {view.league.homeTeamName} ·
          Gast: {view.league.guestClubName} – {view.league.guestTeamName}
        </p>
      {/if}
    </div>
    <span class="spacer"></span>
    <a class="button" href={href(`/competitions/${id}/results`)}>Wettkampfergebnisse</a>
    {#if c.active}
      <a class="button primary" href={href(`/competitions/${id}/entry`)}>+ Beitrag hinzufügen</a>
    {/if}
  </div>

  {#if incomplete.length && c.active}
    <div class="notice">
      <strong>Unvollständige Ergebnisse:</strong>
      {#each incomplete as e (e.matchId)}
        <span class="incomplete">
          {e.shooter} ({e.shots}/{c.count})
          <button onclick={() => navigate(`/competitions/${id}/entry`, { match: e.matchId })}>Fortsetzen</button>
        </span>
      {/each}
    </div>
  {/if}

  <section class="card">
    <h2>Teilnehmer ({view.entries.length})</h2>
    {#if view.entries.length === 0}
      <p class="muted">Noch keine Beiträge.</p>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Schütze</th>
              <th>Verein</th>
              <th>Mannschaft</th>
              <th class="num">Schüsse</th>
              <th>Serien</th>
              <th class="num">Wertung</th>
            </tr>
          </thead>
          <tbody>
            {#each view.entries as e (e.matchId)}
              <tr class="clickable" onclick={() => navigate(`/matches/${e.matchId}`, { competition: id })}>
                <td>{e.shooter} {#if e.active}<span class="badge active">unvollständig</span>{/if}</td>
                <td>{e.club}</td>
                <td>{e.team}</td>
                <td class="num">{e.shots}/{c.count}</td>
                <td class="muted">{e.series.join(' · ')}</td>
                <td class="num"><strong>{e.value}</strong></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
{:else}
  <p class="muted">Lade…</p>
{/if}

<style>
  .head {
    align-items: flex-start;
    margin-bottom: 1rem;
  }
  .head h1 {
    margin: 0;
  }
  .sub {
    margin: 0.2rem 0 0;
  }
  .notice {
    margin-bottom: 1rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1rem;
    align-items: center;
  }
  .incomplete {
    display: inline-flex;
    gap: 0.4rem;
    align-items: center;
  }
  .incomplete button {
    padding: 0.15rem 0.6rem;
  }
</style>
