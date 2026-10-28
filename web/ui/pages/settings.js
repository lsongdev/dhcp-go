import { html, useEffect, useState } from '../lib/html.js';
import { useAction } from '../lib/hooks.js';
import { Page, Notice, Loading, Field, Check, ListInput, KV, toast } from '../lib/ui.js';
import { usePanel } from '../lib/panel.js';
import { useApi, ServerStatus } from '../dhcp.js';

const LEASE_TIMES = [
  [1800, '30 minutes'], [3600, '1 hour'], [6 * 3600, '6 hours'], [12 * 3600, '12 hours'],
  [86400, '1 day'], [3 * 86400, '3 days'], [7 * 86400, '7 days'],
];

const toU32 = ip => ip.split('.').reduce((n, part) => n * 256 + Number(part), 0);
const toIP = n => [24, 16, 8, 0].map(s => Math.floor(n / 2 ** s) % 256).join('.');

// suggestRange proposes a range on an interface address such as
// 192.168.1.10/24: .100 to .200 of the network, or its upper half when the
// network is smaller.
function suggestRange(cidr) {
  const [ip, bits] = cidr.split('/');
  const size = 2 ** (32 - Number(bits));
  if (size < 8) return null;
  const base = toU32(ip) - (toU32(ip) % size);
  return size >= 256 ? [toIP(base + 100), toIP(base + 200)] : [toIP(base + size / 2), toIP(base + size - 2)];
}

export default function Settings() {
  const config = useApi('/config');
  const status = useApi('/status', 10000);
  const interfaces = useApi('/interfaces');
  if (config.data === undefined) {
    return html`<${Page} title="Settings">${config.error ? html`<${Notice} error=${config.error} />` : html`<${Loading} />`}<//>`;
  }
  return html`<${Page} title="Settings" subtitle="The network the DHCP server answers on and what it tells clients.">
    ${status.data ? html`<div class="card status-card"><${KV} items=${[
      ['Status', html`<${ServerStatus} status=${status.data} />`],
      ['Problem', status.data.error || null],
      ['Requests received', status.data.running ? String(status.data.requests) : null],
    ]} /></div>` : null}
    <${SettingsForm} saved=${config.data} interfaces=${interfaces.data || []}
      onSaved=${() => { config.reload(); status.reload(); }} />
  <//>`;
}

function SettingsForm({ saved, interfaces, onSaved }) {
  const { api } = usePanel();
  const [c, setC] = useState(() => ({ ...saved, dns: saved.dns || [], lease_time: saved.lease_time || 12 * 3600 }));
  const save = useAction();
  const set = patch => setC(prev => ({ ...prev, ...patch }));
  const iface = interfaces.find(i => i.name === c.interface);

  // A new configuration gets a range suggested from the interface.
  useEffect(() => {
    if (!iface || c.range_start || c.range_end) return;
    const range = suggestRange(iface.addresses[0]);
    if (range) set({ range_start: range[0], range_end: range[1] });
  }, [iface?.name]);

  const submit = e => {
    e.preventDefault();
    if (c.enabled && !saved.enabled && !window.confirm(
      'Turn on the DHCP server?\n\nOnly one DHCP server should answer on a network. Turn off the DHCP server of your router '
      + '(or of any other device) on this network first; otherwise devices may get conflicting addresses and lose their connection.')) return;
    save.run(async () => {
      await api.put('/config', { ...c, dns: c.dns.filter(Boolean) });
      toast(c.enabled ? 'Saved; the server restarted' : 'Saved', 'success');
      onSaved();
    });
  };
  const leaseKnown = LEASE_TIMES.some(([v]) => v === c.lease_time);
  return html`<form class="form form-narrow" onSubmit=${submit}>
    <${Notice} error=${save.error} />
    <${Check} label="Serve DHCP on this network" checked=${c.enabled} onChange=${e => set({ enabled: e.currentTarget.checked })} />
    <div class="form-grid">
      <${Field} label="Interface" required=${c.enabled}
        hint=${iface ? `Answers on ${iface.addresses.join(', ')}` : 'The network the server answers on; its address is the server address'}>
        <select class="input" value=${c.interface} onChange=${e => set({ interface: e.currentTarget.value })}>
          <option value="">Choose an interface</option>
          ${interfaces.map(i => html`<option key=${i.name} value=${i.name}>${i.name} (${i.addresses.join(', ')})${i.up ? '' : ' · down'}</option>`)}
          ${c.interface && !iface ? html`<option value=${c.interface}>${c.interface} (not found)</option>` : null}
        </select>
      <//>
      <${Field} label="Lease time">
        <select class="input" value=${c.lease_time} onChange=${e => set({ lease_time: Number(e.currentTarget.value) })}>
          ${LEASE_TIMES.map(([v, label]) => html`<option key=${v} value=${v}>${label}</option>`)}
          ${leaseKnown ? null : html`<option value=${c.lease_time}>${c.lease_time} seconds</option>`}
        </select>
      <//>
      <${Field} label="First address" required=${c.enabled}>
        <input class="input mono" value=${c.range_start} placeholder="192.168.1.100" onInput=${e => set({ range_start: e.currentTarget.value })} />
      <//>
      <${Field} label="Last address" required=${c.enabled}>
        <input class="input mono" value=${c.range_end} placeholder="192.168.1.200" onInput=${e => set({ range_end: e.currentTarget.value })} />
      <//>
      <${Field} label="Router" hint="The default gateway clients use; leave empty to send none">
        <input class="input mono" value=${c.router} placeholder="192.168.1.1" onInput=${e => set({ router: e.currentTarget.value })} />
      <//>
      <${Field} label="Domain" hint="Search domain for short host names, e.g. lan">
        <input class="input" value=${c.domain} placeholder="lan" onInput=${e => set({ domain: e.currentTarget.value })} />
      <//>
    </div>
    <${Field} label="DNS servers" hint="In order of preference; leave empty to send none">
      <${ListInput} values=${c.dns} onChange=${dns => set({ dns })} placeholder="192.168.1.1" inputClassName="input mono" />
    <//>
    <div class="form-actions">
      <button class="button button-primary" type="submit" disabled=${save.busy}>${save.busy ? 'Saving…' : 'Save'}</button>
    </div>
  </form>`;
}
