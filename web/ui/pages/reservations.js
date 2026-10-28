import { html, useState } from '../lib/html.js';
import { useBusy } from '../lib/hooks.js';
import { Page, Notice, Loading, Empty, Table, confirmDanger, toastError } from '../lib/ui.js';
import { usePanel } from '../lib/panel.js';
import { useApi, ReservationForm } from '../dhcp.js';

export default function Reservations() {
  const { api } = usePanel();
  const list = useApi('/reservations');
  const [busy, run] = useBusy(toastError);
  const [editing, setEditing] = useState(null); // a reservation, or {} for a new one

  const remove = r => {
    if (!confirmDanger(`Delete the reservation of ${r.ip} for ${r.hostname || r.mac}?`)) return;
    run(r.id, async () => { await api.del(`/reservations/${r.id}`); list.reload(); });
  };

  return html`<${Page} title="Reservations" subtitle="Devices that always get the same address."
    actions=${html`<button class="button button-primary" onClick=${() => setEditing({})}>Add reservation</button>`}>
    <${Notice} error=${list.error} />
    ${list.data === undefined ? (list.error ? null : html`<${Loading} />`) : !list.data.length
      ? html`<${Empty}>No reservations. Reserve a device's current address from its lease, or add one here.<//>` : html`
      <${Table}>
        <thead><tr><th>Address</th><th>Device</th><th>Description</th><th class="actions"></th></tr></thead>
        <tbody>${list.data.map(r => html`<tr key=${r.id}>
          <td class="mono nowrap">${r.ip}</td>
          <td>
            <div class="cell-main">${r.hostname || html`<span class="muted">No host name</span>`}</div>
            <div class="cell-sub mono">${r.mac}</div>
          </td>
          <td class="muted">${r.description || '—'}</td>
          <td class="actions"><span class="button-row">
            <button class="button button-small" onClick=${() => setEditing(r)}>Edit</button>
            <button class="button button-small button-danger" disabled=${busy[r.id]} onClick=${() => remove(r)}>Delete</button>
          </span></td>
        </tr>`)}</tbody>
      <//>`}
    ${editing ? html`<${ReservationForm} reservation=${editing} onClose=${() => setEditing(null)}
      onSaved=${() => { setEditing(null); list.reload(); }} />` : null}
  <//>`;
}
