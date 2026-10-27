<script lang="ts">
  // In-page confirmation dialog.
  let {
    open = $bindable(false),
    title,
    message = '',
    confirmLabel = 'OK',
    danger = false,
    onconfirm,
  }: {
    open?: boolean
    title: string
    message?: string
    confirmLabel?: string
    danger?: boolean
    onconfirm: () => void
  } = $props()

  let dialog: HTMLDialogElement | undefined = $state()

  $effect(() => {
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    if (!open && dialog.open) dialog.close()
  })
</script>

<dialog bind:this={dialog} onclose={() => (open = false)}>
  <h3>{title}</h3>
  {#if message}<p>{message}</p>{/if}
  <div class="actions">
    <button onclick={() => (open = false)}>Abbrechen</button>
    <button
      class={danger ? 'danger' : 'primary'}
      onclick={() => {
        open = false
        onconfirm()
      }}>{confirmLabel}</button
    >
  </div>
</dialog>

<style>
  dialog {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 1.25rem 1.5rem;
    max-width: min(28rem, calc(100vw - 2rem));
    background: var(--surface);
    color: var(--text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.35);
  }
  h3 {
    margin: 0 0 0.5rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1rem;
  }
</style>
