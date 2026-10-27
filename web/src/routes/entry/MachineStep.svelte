<script lang="ts">
  import { api, bytesToBase64, type Machine } from '../../lib/api'

  let {
    machineId = $bindable(),
    data = $bindable(),
    onback,
    onnext,
  }: {
    machineId: string
    data: string | undefined
    onback?: () => void
    onnext: () => void
  } = $props()

  let machines = $state<Machine[] | null>(null)
  let error = $state('')
  let fileName = $state('')
  let qrText = $state('')

  async function load() {
    machines = null
    error = ''
    try {
      machines = await api.machines()
      if (!machines.some((m) => m.id === machineId)) machineId = machines[0]?.id ?? ''
    } catch (e) {
      error = (e as Error).message
      machines = []
    }
  }
  load()

  const machine = $derived(machines?.find((m) => m.id === machineId))

  async function pickFile(e: Event) {
    const file = (e.currentTarget as HTMLInputElement).files?.[0]
    data = undefined
    fileName = ''
    if (!file) return
    data = bytesToBase64(new Uint8Array(await file.arrayBuffer()))
    fileName = file.name
  }

  $effect(() => {
    if (machine?.input === 'text') data = qrText.trim() ? bytesToBase64(new TextEncoder().encode(qrText.trim())) : undefined
  })

  function select(id: string) {
    if (id !== machineId) {
      machineId = id
      data = undefined
      fileName = ''
    }
  }

  const ready = $derived(!!machine && (machine.input === '' || !!data))
</script>

<section class="card">
  <div class="actions">
    <h2>Gerät auswählen</h2>
    <span class="spacer"></span>
    <button onclick={load} disabled={machines === null}>Neu suchen</button>
  </div>

  {#if machines === null}
    <p class="muted">Suche angeschlossene Geräte…</p>
  {:else}
    {#if error}<p class="error">{error}</p>{/if}
    <div class="machines" role="radiogroup" aria-label="Gerät">
      {#each machines as m (m.id)}
        <label class="machine" class:selected={m.id === machineId}>
          <input type="radio" name="machine" checked={m.id === machineId} onchange={() => select(m.id)} />
          <span>
            <strong>{m.name}</strong>
            <small class="muted"
              >{m.input === 'file' ? `Datei (${m.accept})` : m.input === 'text' ? 'Texteingabe' : 'Auswertemaschine'}</small
            >
          </span>
        </label>
      {/each}
    </div>
    {#if !machines.some((m) => m.input === '')}
      <p class="muted">Keine Auswertemaschine gefunden. Gerät anschließen und „Neu suchen“ klicken.</p>
    {/if}

    {#if machine?.input === 'file'}
      <label class="field input">
        <span>Datei</span>
        <input type="file" accept={machine.accept} onchange={pickFile} />
        {#if fileName}<small class="muted">{fileName}</small>{/if}
      </label>
    {:else if machine?.input === 'text'}
      <label class="field input">
        <span>QR-Code-Inhalt</span>
        <textarea bind:value={qrText} rows="3" placeholder="z.B. 04092023LGA4132.4+1.25-0.42…"></textarea>
      </label>
    {/if}
  {/if}

  <div class="actions end">
    {#if onback}<button onclick={onback}>Zurück</button>{/if}
    <button class="primary" disabled={!ready} onclick={onnext}>Einlesen starten</button>
  </div>
</section>

<style>
  h2 {
    margin: 0;
  }
  .machines {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr));
    gap: 0.6rem;
    margin: 1rem 0;
  }
  .machine {
    display: flex;
    gap: 0.6rem;
    align-items: flex-start;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 0.6rem 0.8rem;
    cursor: pointer;
  }
  .machine span {
    display: flex;
    flex-direction: column;
  }
  .machine.selected {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }
  .input {
    max-width: 36rem;
    margin-bottom: 1rem;
  }
  .end {
    justify-content: flex-end;
  }
</style>
