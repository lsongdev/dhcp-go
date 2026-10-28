// DHCP server entry point (panel module contract, version 1). The host is
// a panel shell, or standalone.js on its own.
import { createMount } from './lib/mount.js';

const routes = [
  { path: '/', load: () => import('./pages/leases.js') },
  { path: '/reservations', load: () => import('./pages/reservations.js') },
  { path: '/activity', load: () => import('./pages/activity.js') },
  { path: '/settings', load: () => import('./pages/settings.js') },
];

export const mount = createMount({ routes });
