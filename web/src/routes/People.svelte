<script lang="ts">
  // Master data: shooters, clubs and teams.
  import { api, type Named, type UserSummary } from '../lib/api'
  import { t, tx } from '../lib/i18n.svelte'
  import { href } from '../lib/router.svelte'

  type Tab = 'users' | 'clubs' | 'teams'
  let tab = $state<Tab>('users')
  let users = $state<UserSummary[]>([])
  let clubs = $state<Named[]>([])
  let teams = $state<Named[]>([])
  let search = $state('')
  let error = $state('')

  // Row being edited; id is empty for a new row.
  let editing = $state<{ id: string; name: string; birthday: string } | null>(null)

  async function reload() {
    ;[users, clubs, teams] = await Promise.all([api.users(), api.clubs(), api.teams()])
  }
  reload().catch((e) => (error = e.message))

  const rows = $derived(
    (tab === 'users' ? users : tab === 'clubs' ? clubs : teams).filter((r) =>
      r.name.toLowerCase().includes(search.toLowerCase()),
    ) as (Named & Partial<UserSummary>)[],
  )

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing) return
    error = ''
    try {
      const id = editing.id || undefined
      if (tab === 'users') await api.saveUser({ id, name: editing.name, birthday: editing.birthday })
      else if (tab === 'clubs') await api.saveClub({ id, name: editing.name })
      else await api.saveTeam({ id, name: editing.name })
      editing = null
      await reload()
    } catch (err) {
      error = (err as Error).message
    }
  }

  const labels = { users: 'Schützen', clubs: 'Vereine', teams: 'Mannschaften' } as const
</script>

<h1>{t('Stammdaten')}</h1>

<div class="actions bar">
  <div class="tabs" role="tablist">
    {#each Object.entries(labels) as [key, label]}
      <button
        role="tab"
        aria-selected={tab === key}
        class:current={tab === key}
        onclick={() => ((tab = key as Tab), (editing = null))}>{t(label)}</button
      >
    {/each}
  </div>
  <span class="spacer"></span>
  <input type="search" placeholder={t('Suchen…')} bind:value={search} />
  <button class="primary" onclick={() => (editing = { id: '', name: '', birthday: '' })}>{t('+ Neu')}</button>
</div>

{#if error}<p class="error">{tx(error)}</p>{/if}

{#snippet form()}
  <tr class="editing">
    <td colspan="4">
      <form onsubmit={save} class="inline">
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={editing!.name} placeholder={t('Name')} required autofocus />
        {#if tab === 'users'}<input bind:value={editing!.birthday} placeholder={t('Geburtsdatum')} />{/if}
        <button type="submit" class="primary">{t('Speichern')}</button>
        <button type="button" onclick={() => (editing = null)}>{t('Abbrechen')}</button>
      </form>
    </td>
  </tr>
{/snippet}

<section class="card">
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th>{t('Name')}</th>
          {#if tab === 'users'}
            <th>{t('Geburtsdatum')}</th>
            <th class="num">{t('Ergebnisse')}</th>
          {/if}
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#if editing && !editing.id}{@render form()}{/if}
        {#each rows as r (r.id)}
          {#if editing?.id === r.id}
            {@render form()}
          {:else}
            <tr>
              {#if tab === 'users'}
                <td><a href={href(`/people/${r.id}`)} title={t('Ergebnisse anzeigen')}>{r.name}</a></td>
                <td>{r.birthday}</td>
                <td class="num"><a href={href(`/people/${r.id}`)}>{r.matchCount ?? 0}</a></td>
              {:else}
                <td>{r.name}</td>
              {/if}
              <td class="num">
                <button onclick={() => (editing = { id: r.id, name: r.name, birthday: r.birthday ?? '' })}
                  >{t('Bearbeiten')}</button
                >
              </td>
            </tr>
          {/if}
        {:else}
          <tr><td colspan="4" class="muted">{t('Keine Einträge.')}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
</section>

<style>
  .bar {
    margin-bottom: 1rem;
  }
  .tabs {
    display: flex;
    gap: 0.25rem;
  }
  .tabs button.current {
    background: var(--accent);
    color: var(--accent-text);
    border-color: var(--accent);
  }
  .inline {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  td button {
    padding: 0.2rem 0.7rem;
  }
</style>
