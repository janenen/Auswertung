<script lang="ts">
  // All results of one shooter, competitions and quick analyses, newest first.
  import { api, type ShooterMatch, type User } from '../lib/api'
  import { result } from '../lib/meta.svelte'
  import { href, navigate } from '../lib/router.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let { id }: { id: string } = $props()

  let user = $state<User | null>(null)
  let matches = $state<ShooterMatch[]>([])
  let error = $state('')
  let kind = $state<'all' | 'competitions' | 'quick'>('all')
  let target = $state('')

  $effect(() => {
    api
      .userMatches(id)
      .then((r) => ((user = r.user), (matches = r.matches)))
      .catch((e) => (error = e.message))
  })

  const targets = $derived([...new Set(matches.map((m) => m.typeOfTarget))].sort())
  const shown = $derived(
    matches.filter(
      (m) =>
        (kind === 'all' || (kind === 'quick') === !m.competitionId) && (target === '' || m.typeOfTarget === target),
    ),
  )
</script>

{#if error}
  <p class="error">{tx(error)}</p>
{:else if user}
  <div class="actions head">
    <div>
      <h1>{user.name}</h1>
      <p class="muted sub">
        {#if user.birthday}{t('Geburtsdatum')}: {user.birthday} ·{/if}
        {t(matches.length === 1 ? '{n} Ergebnis' : '{n} Ergebnisse', { n: matches.length })}
      </p>
    </div>
    <span class="spacer"></span>
    <a class="button" href={href('/people')}>{t('Zurück')}</a>
  </div>

  {#if matches.length}
    <div class="actions filters">
      <label class="field">
        <span>{t('Disziplin')}</span>
        <select bind:value={target}>
          <option value="">{t('Alle Disziplinen')}</option>
          {#each targets as type}<option value={type}>{type}</option>{/each}
        </select>
      </label>
      <label class="field">
        <span>{t('Art')}</span>
        <select bind:value={kind}>
          <option value="all">{t('Alle Ergebnisse')}</option>
          <option value="competitions">{t('Wettkämpfe')}</option>
          <option value="quick">{t('Schnellauswertungen')}</option>
        </select>
      </label>
    </div>
  {/if}

  <section class="card">
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t('Datum')}</th>
            <th>{t('Wettkampf')}</th>
            <th>{t('Scheibe')}</th>
            <th class="num">{t('Schüsse')}</th>
            <th>{t('Serien')}</th>
            <th class="num">{t('Ergebnis')}</th>
          </tr>
        </thead>
        <tbody>
          {#each shown as m (m.matchId)}
            <tr class="clickable" onclick={() => navigate(`/matches/${m.matchId}`, { shooter: id })}>
              <td>{m.date}</td>
              <td>
                {tx(m.title)}
                {#if !m.competitionId}<span class="badge">{t('Schnellauswertung')}</span>{/if}
                {#if m.active}<span class="badge active">{t('unvollständig')}</span>{/if}
              </td>
              <td>{m.typeOfTarget}</td>
              <td class="num">{m.count ? `${m.shots}/${m.count}` : m.shots}</td>
              <td class="muted">{m.series.join(' · ')}</td>
              <td class="num">
                <strong>{result(m, m.decimal)}</strong>
                <small class="muted">({result(m, !m.decimal)})</small>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="muted">
                {t(matches.length ? 'Keine Ergebnisse für diese Auswahl.' : 'Noch keine Ergebnisse.')}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>
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
  .filters {
    margin-bottom: 0.75rem;
    align-items: flex-end;
  }
  .filters .field {
    min-width: 12rem;
  }
</style>
