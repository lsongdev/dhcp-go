import { html, useState } from '../lib/html.js';
import { useBusy } from '../lib/hooks.js';
import { Page, Notice, Loading, Empty, Tabs, Table, KV, Badge, confirmDanger, toastError } from '../lib/ui.js';
import { usePanel } from '../lib/panel.js';
import { ago, datetime } from '../lib/format.js';
import { useApi, ServerStatus, ServerNotice, ReservationForm } from '../dhcp.js';

export default function Leases() {
  const { api, query, setQuery } = usePanel();
  const status = useApi('/status', 10000);
  const leases = useApi('/leases', 10000);
  const [busy, run] = useBusy(toastError);
  const [reserving, setReserving] = useState(null);

  const all = leases.data || [];
  const show = query.show || 'active';
  const text = (query.q || '').trim().toLowerCase();
  const matches = all.filter(l => show === 'all' || l.active)
    .filter(l => !text || [l.ip, l.mac, l.hostname].some(v => v?.toLowerCase().includes(text)));
  const active = all.filter(l => l.active).length;

  const remove = l => {
    if (!confirmDanger(`Forget the lease of ${l.ip}${l.hostname ? ` (${l.hostname})` : ''}? The address can then go to another device; a device still using it is refused when it renews.`)) return;
    run(l.ip, async () => { await api.del(`/leases/${encodeURIComponent(l.ip)}`); leases.reload(); status.reload(); });
  };

  const s = status.data;
  return html`<${Page} title="Leases" subtitle="Addresses the DHCP server handed out.">
    <${Notice} error=${status.error || leases.error} />
    <${ServerNotice} status=${s} />
    ${s?.running ? html`<div class="card status-card"><${KV} items=${[
      ['Status', html`<${ServerStatus} status=${s} />`],
      ['Interface', html`${s.interface} <span class="muted">· ${s.server_ip} on ${s.subnet}</span>`],
      ['Range', html`<span class="mono">${s.range_start} – ${s.range_end}</span>`],
      ['In use', `${s.active_leases} of ${s.pool_size} addresses`],
      ['Reservations', String(s.reservations)],
    ]} /></div>` : null}
    <${Tabs} active=${show} onChange=${id => setQuery({ show: id === 'active' ? '' : id })}
      tabs=${[['active', `Active (${active})`], ['all', `All (${all.length})`]]} />
    <div class="toolbar">
      <input class="input" type="search" placeholder="Filter by address, MAC or host name" aria-label="Filter"
        value=${query.q || ''} onInput=${e => setQuery({ q: e.currentTarget.value })} />
    </div>
    ${leases.data === undefined ? (leases.error ? null : html`<${Loading} />`) : matches.length === 0 ? html`<${Empty}>${all.length ? 'No lease matches.' : 'No leases yet.'}<//>` : html`
      <${Table}>
        <thead><tr><th>Address</th><th>Device</th><th>Expires</th><th class="actions"></th></tr></thead>
        <tbody>${matches.map(l => html`<tr key=${l.ip}>
          <td class="mono nowrap">${l.ip}</td>
          <td>
            <div class="cell-main">${l.hostname || html`<span class="muted">No host name</span>`}
              ${l.reserved ? html` <${Badge} tone="blue">Reserved<//>` : null}</div>
            <div class="cell-sub mono">${l.mac}</div>
          </td>
          <td class="nowrap" title=${datetime(l.expires_at)}>${l.active ? ago(l.expires_at) : html`<span class="muted">Expired ${ago(l.expires_at)}</span>`}</td>
          <td class="actions"><span class="button-row">
            ${l.reserved ? null : html`<button class="button button-small" onClick=${() => setReserving(l)}>Reserve</button>`}
            <button class="button button-small button-danger" disabled=${busy[l.ip]} onClick=${() => remove(l)}>Forget</button>
          </span></td>
        </tr>`)}</tbody>
      <//>`}
    ${reserving ? html`<${ReservationForm} reservation=${{ mac: reserving.mac, ip: reserving.ip, hostname: reserving.hostname }}
      onClose=${() => setReserving(null)} onSaved=${() => { setReserving(null); leases.reload(); status.reload(); }} />` : null}
  <//>`;
}
