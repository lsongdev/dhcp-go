import { html } from '@lsongdev/panel/html.js';
import { PageHeader, Alert, Loading, Empty, Table, Badge } from '@lsongdev/scripts/react/components/index.js';
import { datetime } from '@lsongdev/panel/format.js';
import { useApi, ServerNotice } from '../dhcp.js';

const RESULT_TONES = { offer: 'info', ack: 'success', nak: 'danger', ignored: undefined };
const capitalize = s => s[0].toUpperCase() + s.slice(1);

export default function Activity() {
  const status = useApi('/status', 10000);
  const events = useApi('/events', 3000);
  return html`<${PageHeader} title="Activity" subtitle="Recent requests from clients, newest first. Kept in memory until the server restarts." />
    <${Alert} error=${events.error} />
    <${ServerNotice} status=${status.data} />
    ${events.data === undefined ? (events.error ? null : html`<${Loading} />`) : !events.data.length ? html`<${Empty}>No requests yet.<//>` : html`
      <${Table}>
        <thead><tr><th>Time</th><th>Client</th><th>Request</th><th>Answer</th><th>Address</th><th>Detail</th></tr></thead>
        <tbody>${events.data.map((e, i) => html`<tr key=${i}>
          <td class="nowrap muted">${datetime(e.time)}</td>
          <td><div class="cell-main">${e.hostname || html`<span class="mono">${e.mac}</span>`}</div>
            ${e.hostname ? html`<div class="cell-sub mono">${e.mac}</div>` : null}</td>
          <td>${capitalize(e.type)}</td>
          <td>${e.result ? html`<${Badge} variant=${RESULT_TONES[e.result]}>${e.result.toUpperCase()}<//>` : html`<span class="muted">—</span>`}</td>
          <td class="mono nowrap">${e.ip || '—'}</td>
          <td class="muted small">${e.detail || ''}</td>
        </tr>`)}</tbody>
      <//>`}`;
}
