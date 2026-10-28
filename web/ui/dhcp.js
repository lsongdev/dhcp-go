// DHCP UI helpers shared by the pages.
import { html, useState } from './lib/html.js';
import { useAction, useResource } from './lib/hooks.js';
import { usePanel } from './lib/panel.js';
import { Notice, Status, Field, Modal } from './lib/ui.js';

// useApi loads a GET endpoint, polling every interval milliseconds when
// given. A null path loads nothing yet.
export function useApi(path, interval) {
  const { api } = usePanel();
  return useResource(signal => (path ? api.get(path, { signal }) : Promise.resolve(undefined)), [path], { interval });
}

// ServerStatus shows whether the server is off, serving, or failing.
export function ServerStatus({ status }) {
  if (!status.enabled) return html`<${Status} state="stopped" label="Off" />`;
  if (status.running) return html`<${Status} state="running" label="Serving" />`;
  return html`<${Status} state="failed" label="Not running" title=${status.error} />`;
}

// ServerNotice explains a server that is off or failing, linking to the
// settings.
export function ServerNotice({ status }) {
  const { href } = usePanel();
  if (!status || status.running) return null;
  if (!status.enabled) {
    return html`<${Notice} tone="info">The DHCP server is off. Set it up and turn it on in <a href=${href('/settings')}>Settings</a>.<//>`;
  }
  return html`<${Notice} tone="error">The DHCP server is not running: ${status.error || 'unknown error'}. It retries every 30 seconds; check <a href=${href('/settings')}>Settings</a>.<//>`;
}

// ReservationForm adds a reservation, or edits one when it has an id. A
// lease passed as the reservation prefills it.
export function ReservationForm({ reservation, onClose, onSaved }) {
  const { api } = usePanel();
  const isNew = !reservation.id;
  const [r, setR] = useState({
    mac: reservation.mac || '', ip: reservation.ip || '',
    hostname: reservation.hostname || '', description: reservation.description || '',
  });
  const save = useAction();
  const set = patch => setR(prev => ({ ...prev, ...patch }));
  const submit = e => {
    e.preventDefault();
    save.run(async () => {
      if (isNew) await api.post('/reservations', r);
      else await api.put(`/reservations/${reservation.id}`, r);
      onSaved();
    });
  };
  return html`<${Modal} title=${isNew ? 'Add reservation' : 'Edit reservation'} onClose=${onClose} footer=${html`
    <button class="button" type="button" onClick=${onClose}>Cancel</button>
    <button class="button button-primary" type="submit" form="reservation-form" disabled=${save.busy}>${isNew ? 'Add' : 'Save'}</button>`}>
    <form id="reservation-form" class="form" onSubmit=${submit}>
      <${Notice} error=${save.error} />
      <div class="form-grid">
        <${Field} label="MAC address" required>
          <input class="input mono" required value=${r.mac} placeholder="aa:bb:cc:dd:ee:ff" autofocus=${isNew && !r.mac}
            onInput=${e => set({ mac: e.currentTarget.value })} />
        <//>
        <${Field} label="IP address" required hint="Inside or outside the range, on the server's network">
          <input class="input mono" required value=${r.ip} placeholder="192.168.1.20" onInput=${e => set({ ip: e.currentTarget.value })} />
        <//>
        <${Field} label="Host name">
          <input class="input" value=${r.hostname} placeholder="printer" onInput=${e => set({ hostname: e.currentTarget.value })} />
        <//>
        <${Field} label="Description">
          <input class="input" value=${r.description} onInput=${e => set({ description: e.currentTarget.value })} />
        <//>
      </div>
      <p class="muted small">A device keeps its current address until it renews its lease, then moves to the reserved one.</p>
    </form>
  <//>`;
}
