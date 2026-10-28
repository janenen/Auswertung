<script lang="ts">
  import { api, type CompetitionView, type Entry } from '../lib/api'
  import { href, navigate } from '../lib/router.svelte'
  import { t, tx } from '../lib/i18n.svelte'
  import { meta } from '../lib/meta.svelte'
  import Confirm from '../lib/Confirm.svelte'

  let { id }: { id: string } = $props()

  let view = $state<CompetitionView | null>(null)
  let error = $state('')
  let simulating = $state(false)
  // Entry to remove after confirmation, e.g. one recorded for the wrong shooter.
  let removing = $state<Entry | null>(null)
  let confirmRemove = $state(false)

  function load() {
    api
      .competition(id)
      .then((v) => (view = v))
      .catch((e) => (error = e.message))
  }
  $effect(load)

  async function simulate() {
    simulating = true
    try {
      await api.simulateLeague(id)
      load()
    } catch (e) {
      error = (e as Error).message
    } finally {
      simulating = false
    }
  }

  async function remove(e: Entry) {
    try {
      await api.deleteMatch(e.matchId)
      load()
    } catch (err) {
      error = (err as Error).message
    }
  }

  const c = $derived(view?.competition)
  const incomplete = $derived(view?.entries.filter((e) => e.active) ?? [])
</script>

{#if error}
  <p class="error">{tx(error)}</p>
{:else if view && c}
  <div class="actions head">
    <div>
      <h1>{c.name}</h1>
      <p class="muted sub">
        {c.date} · {t('Scheibe {type}', { type: c.type_of_target })} · {t('{n} Schuss', { n: c.count })} ·
        {t('{n} pro Scheibe', { n: c.shots_per_target })} · {tx(c.modus)}
        {#if !c.active}<span class="badge">{t('beendet')}</span>{/if}
      </p>
      {#if view.league}
        <p class="muted sub">
          {tx(view.league.league)}
          {view.league.group} · {view.league.discipline} ·
          {t('Heim: {club} – {team}', { club: view.league.homeClubName, team: view.league.homeTeamName })} ·
          {t('Gast: {club} – {team}', { club: view.league.guestClubName, team: view.league.guestTeamName })}
        </p>
      {/if}
    </div>
    <span class="spacer"></span>
    <a class="button" href={href(`/competitions/${id}/results`)}>{t('Wettkampfergebnisse')}</a>
    {#if c.active && view.league && meta.simulator}
      <button onclick={simulate} disabled={simulating} title={t('Erzeugt je 3 zufällige Ergebnisse für Heim und Gast')}
        >{t('Ergebnisse simulieren')}</button
      >
    {/if}
    {#if c.active}
      <a class="button primary" href={href(`/competitions/${id}/entry`)}>{t('+ Beitrag hinzufügen')}</a>
    {/if}
  </div>

  {#if incomplete.length && c.active}
    <div class="notice">
      <strong>{t('Unvollständige Ergebnisse:')}</strong>
      {#each incomplete as e (e.matchId)}
        <span class="incomplete">
          {e.shooter} ({e.shots}/{c.count})
          <button onclick={() => navigate(`/competitions/${id}/entry`, { match: e.matchId })}>{t('Fortsetzen')}</button>
        </span>
      {/each}
    </div>
  {/if}

  <section class="card">
    <h2>{t('Teilnehmer ({n})', { n: view.entries.length })}</h2>
    {#if view.entries.length === 0}
      <p class="muted">{t('Noch keine Beiträge.')}</p>
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{t('Schütze')}</th>
              <th>{t('Verein')}</th>
              <th>{t('Mannschaft')}</th>
              <th class="num">{t('Schüsse')}</th>
              <th>{t('Serien')}</th>
              <th class="num">{t('Wertung')}</th>
              {#if c.active}<th></th>{/if}
            </tr>
          </thead>
          <tbody>
            {#each view.entries as e (e.matchId)}
              <tr class="clickable" onclick={() => navigate(`/matches/${e.matchId}`, { competition: id })}>
                <td>{e.shooter} {#if e.active}<span class="badge active">{t('unvollständig')}</span>{/if}</td>
                <td>{e.club}</td>
                <td>{e.team}</td>
                <td class="num">{e.shots}/{c.count}</td>
                <td class="muted">{e.series.join(' · ')}</td>
                <td class="num"><strong>{e.value}</strong></td>
                {#if c.active}
                  <td class="num">
                    <button
                      class="remove"
                      title={t('Beitrag entfernen')}
                      aria-label={t('Beitrag entfernen')}
                      onclick={(ev) => {
                        ev.stopPropagation()
                        removing = e
                        confirmRemove = true
                      }}>✕</button
                    >
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  <Confirm
    bind:open={confirmRemove}
    title={t('Beitrag aus dem Wettkampf entfernen?')}
    message={removing
      ? t('{shooter}, {n} Schuss. Das Ergebnis wird endgültig gelöscht.', { shooter: removing.shooter, n: removing.shots })
      : ''}
    confirmLabel={t('Entfernen')}
    danger
    onconfirm={() => removing && remove(removing)}
  />
{:else}
  <p class="muted">{t('Lade…')}</p>
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
  .incomplete button,
  .remove {
    padding: 0.15rem 0.6rem;
  }
</style>
