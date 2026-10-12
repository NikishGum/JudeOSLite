let mutations = 0;
let reloading = false;
export const pendingMutations = () => mutations;
export function beginMutation() {
  if (reloading) throw new Error('Application is reloading');
  mutations += 1; window.dispatchEvent(new Event('mutation-state'));
}
export function endMutation() {
  mutations -= 1; window.dispatchEvent(new Event('mutation-state'));
}
export function beginAppReload() {
  if (mutations || reloading) return false;
  reloading = true; return true;
}
export function cancelAppReload() { reloading = false; }

// This channel contains no employee IDs, tokens, or records.
const sessionChannel = typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel('pride-session');
sessionChannel?.addEventListener('message', () => window.dispatchEvent(new Event('session-changed')));
export function notifySessionChanged() { sessionChannel?.postMessage('changed'); }
