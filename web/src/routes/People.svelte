<script lang="ts">
  // Master data: shooters, clubs, teams and cameras.
  import { api, cameraSnapshotURL, type Camera, type Named, type UserSummary } from '../lib/api'
  import Confirm from '../lib/Confirm.svelte'
  import { t, tx } from '../lib/i18n.svelte'
  import { href } from '../lib/router.svelte'

  type Tab = 'users' | 'clubs' | 'teams' | 'cameras'
  let tab = $state<Tab>('users')
  let users = $state<UserSummary[]>([])
  let clubs = $state<Named[]>([])
  let teams = $state<Named[]>([])
  let cameras = $state<Camera[]>([])
  let search = $state('')
  let error = $state('')

  // Row being edited; id is empty for a new row.
  let editing = $state<{ id: string; name: string; birthday: string; url: string } | null>(null)
  let deleting = $state<Camera | null>(null)
  let confirmDelete = $state(false)

  async function reload() {
    ;[users, clubs, teams, cameras] = await Promise.all([api.users(), api.clubs(), api.teams(), api.cameras()])
  }
  reload().catch((e) => (error = e.message))

  const rows = $derived(
    (tab === 'users' ? users : tab === 'clubs' ? clubs : tab === 'teams' ? teams : cameras).filter((r) =>
      r.name.toLowerCase().includes(search.toLowerCase()),
    ) as (Named & Partial<UserSummary> & Partial<Camera>)[],
  )

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing) return
    error = ''
    try {
      const id = editing.id || undefined
      if (tab === 'users') await api.saveUser({ id, name: editing.name, birthday: editing.birthday })
      else if (tab === 'clubs') await api.saveClub({ id, name: editing.name })
      else if (tab === 'teams') await api.saveTeam({ id, name: editing.name })
      else await api.saveCamera({ id, name: editing.name, url: editing.url })
      editing = null
      await reload()
    } catch (err) {
      error = (err as Error).message
    }
  }

  async function remove(c: Camera) {
    error = ''
    try {
      await api.deleteCamera(c.id)
      await reload()
    } catch (err) {
      error = (err as Error).message
    }
  }

  const labels = { users: 'Schützen', clubs: 'Vereine', teams: 'Mannschaften', cameras: 'Kameras' } as const
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
  {#if tab === 'cameras'}
    <a class="button" href={href('/cameras')} target="_blank">{t('Kameraansicht öffnen')}</a>
  {/if}
  <input type="search" placeholder={t('Suchen…')} bind:value={search} />
  <button class="primary" onclick={() => (editing = { id: '', name: '', birthday: '', url: '' })}>{t('+ Neu')}</button>
</div>

{#if error}<p class="error">{tx(error)}</p>{/if}

{#snippet form()}
  <tr class="editing">
    <td colspan="4">
      <form onsubmit={save} class="inline">
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={editing!.name} placeholder={t('Name')} required autofocus />
        {#if tab === 'users'}<input bind:value={editing!.birthday} placeholder={t('Geburtsdatum')} />{/if}
        {#if tab === 'cameras'}
          <input class="url" bind:value={editing!.url} placeholder={t('IP-Adresse oder Stream-URL')} required />
        {/if}
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
          {:else if tab === 'cameras'}
            <th>{t('Adresse')}</th>
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
              {:else if tab === 'cameras'}
                <td>{r.name}</td>
                <td><code>{r.url}</code></td>
              {:else}
                <td>{r.name}</td>
              {/if}
              <td class="num">
                {#if tab === 'cameras'}
                  <a class="button" href={cameraSnapshotURL(r.id)} target="_blank">{t('Vorschau')}</a>
                {/if}
                <button
                  onclick={() => (editing = { id: r.id, name: r.name, birthday: r.birthday ?? '', url: r.url ?? '' })}
                  >{t('Bearbeiten')}</button
                >
                {#if tab === 'cameras'}
                  <button onclick={() => ((deleting = { id: r.id, name: r.name, url: r.url ?? '' }), (confirmDelete = true))}>{t('Löschen')}</button>
                {/if}
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

{#if tab === 'cameras'}
  <p class="muted hint">
    {t(
      'Für Kameras mit der Auswertung-Firmware (ESP32-CAM) genügt die IP-Adresse; andere Kameras mit der vollständigen MJPEG-Stream-URL eintragen.',
    )}
  </p>
{/if}

<Confirm
  bind:open={confirmDelete}
  title={t('Kamera löschen?')}
  message={deleting ? t('Die Kamera „{name}" wird entfernt.', { name: deleting.name }) : ''}
  confirmLabel={t('Löschen')}
  danger
  onconfirm={() => {
    if (deleting) remove(deleting)
  }}
/>

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
  .inline .url {
    flex: 1;
    min-width: 16rem;
  }
  td button,
  td .button {
    padding: 0.2rem 0.7rem;
  }
  .hint {
    font-size: 0.9rem;
  }
</style>
