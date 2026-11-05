<script lang="ts">
  // Wizard for a new entry: shooter → machine → live reading. Used for
  // competitions (id), continuing an incomplete match (query.match, of a
  // competition or a quick analysis) and quick analyses (quick, settings in
  // the query).
  import { api, type CompetitionView, type MatchView, type Named, type QuickSettings, type User } from '../lib/api'
  import { navigate, href } from '../lib/router.svelte'
  import ShooterStep from './entry/ShooterStep.svelte'
  import MachineStep from './entry/MachineStep.svelte'
  import ReadingStep from './entry/ReadingStep.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let { id = '', query = {}, quick = false }: { id?: string; query?: Record<string, string>; quick?: boolean } =
    $props()

  const continueMatch = $derived(query.match ?? '')
  // An incomplete quick analysis continues with the settings stored with it.
  const continueQuick = $derived(!id && !!continueMatch)
  const quickSettings: QuickSettings | undefined = $derived(
    quick && !continueQuick
      ? {
          name: query.name || 'Schnellauswertung',
          count: Number(query.count) || 40,
          shots_per_target: Number(query.spt) || 1,
          type_of_target: query.target || 'LG',
          decimal: query.decimal === 'true',
        }
      : undefined,
  )
  const quickDecimal = $derived(query.decimal === 'true')

  let view = $state<CompetitionView | null>(null)
  let quickMatch = $state<MatchView | null>(null)
  let error = $state('')
  // The page is re-created on every route change, so the initial query decides the first step.
  // svelte-ignore state_referenced_locally
  let step = $state<'shooter' | 'machine' | 'reading'>(query.match ? 'machine' : 'shooter')

  // Selection
  let user = $state<User | null>(null)
  let clubId = $state('')
  let teamId = $state('')
  let machineId = $state('')
  let data = $state<string | undefined>()

  $effect(() => {
    if (!id) return
    api
      .competition(id)
      .then((v) => (view = v))
      .catch((e) => (error = e.message))
  })
  $effect(() => {
    if (!continueQuick) return
    api
      .match(continueMatch)
      .then((m) => (quickMatch = m))
      .catch((e) => (error = e.message))
  })

  const competition = $derived(view?.competition)
  const quickContinued = $derived(
    quickMatch && { shooter: quickMatch.shooter, shots: quickMatch.match.shots.length },
  )
  const continued = $derived(view?.entries.find((e) => e.matchId === continueMatch) ?? quickContinued)
  const targetType = $derived(
    competition?.type_of_target ?? quickMatch?.match.type_of_target ?? quickSettings?.type_of_target ?? 'LG',
  )
  const total = $derived(competition?.count ?? quickMatch?.match.count ?? quickSettings?.count ?? 0)
  const already = $derived(continued?.shots ?? 0)
  const title = $derived(
    competition?.name ?? (quickMatch ? quickMatch.match.title || 'Schnellauswertung' : quickSettings?.name) ?? '',
  )
  const backHref = $derived(
    id ? href(`/competitions/${id}`) : continueQuick ? href(`/matches/${continueMatch}`) : href('/quick'),
  )

  function finished(matchId: string) {
    if (continueQuick) {
      navigate(`/matches/${matchId}`)
    } else if (quick) {
      navigate(`/matches/${matchId}`, { title: quickSettings!.name, decimal: quickDecimal })
    } else {
      navigate(`/matches/${matchId}`, { competition: id })
    }
  }

  let clubs = $state<Named[]>([])
  let teams = $state<Named[]>([])
</script>

{#if error}
  <p class="error">{tx(error)}</p>
{:else if (id && !view) || (continueQuick && !quickMatch)}
  <p class="muted">{t('Lade…')}</p>
{:else}
  <div class="actions head">
    <div>
      <h1>{t(continueMatch ? 'Ergebnis fortsetzen' : 'Neuer Beitrag')}</h1>
      <p class="muted sub">
        {tx(title)} · {t('Scheibe {type}', { type: targetType })} · {t('{n} Schuss', { n: total })}
        {#if continued}· {t('{shooter}: {n} von {total} bereits eingelesen', { shooter: continued.shooter, n: already, total })}{/if}
      </p>
    </div>
    <span class="spacer"></span>
    {#if step !== 'reading'}<a class="button" href={backHref}>{t('Abbrechen')}</a>{/if}
  </div>

  <ol class="steps">
    {#if !continueMatch}<li class:current={step === 'shooter'}>{t('Schütze')}</li>{/if}
    <li class:current={step === 'machine'}>{t('Gerät')}</li>
    <li class:current={step === 'reading'}>{t('Einlesen')}</li>
  </ol>

  {#if step === 'shooter'}
    <ShooterStep
      league={view?.league ?? null}
      bind:user
      bind:clubId
      bind:teamId
      bind:clubs
      bind:teams
      onnext={() => (step = 'machine')}
    />
  {:else if step === 'machine'}
    <MachineStep
      bind:machineId
      bind:data
      onback={continueMatch ? undefined : () => (step = 'shooter')}
      onnext={() => (step = 'reading')}
    />
  {:else}
    <ReadingStep
      request={{
        machineId,
        data,
        competitionId: id || undefined,
        quick: quickSettings,
        matchId: continueMatch || undefined,
        userId: user?.id,
        clubId,
        teamId,
      }}
      {targetType}
      {total}
      {already}
      onfinished={finished}
      onback={() => (step = 'machine')}
    />
  {/if}
{/if}

<style>
  .head {
    align-items: flex-start;
    margin-bottom: 0.75rem;
  }
  .head h1 {
    margin: 0;
  }
  .sub {
    margin: 0.2rem 0 0;
  }
  .steps {
    display: flex;
    gap: 0.5rem;
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
    counter-reset: step;
  }
  .steps li {
    counter-increment: step;
    color: var(--muted);
    padding: 0.25rem 0.75rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    font-size: 0.9rem;
  }
  .steps li::before {
    content: counter(step) '. ';
  }
  .steps li.current {
    color: var(--accent-text);
    background: var(--accent);
    border-color: var(--accent);
  }
</style>
