import type { Section } from '../../app/navigation';

// Inline line icons from the selected Working Panel reference; no icon font/runtime.
export function NavigationIcon({ section }: { section: Section | 'more' }) {
  const paths: Record<Section | 'more', string> = {
    home: 'm3 10 9-7 9 7v10H3z M9 20v-7h6v7',
    sessions: 'M5 5h14a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2 M7 3v4m10-4v4M3 11h18',
    participants: 'M12 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0 M3 21v-3a6 6 0 0 1 12 0v3m2-16a3 3 0 0 1 0 6m2 3a5 5 0 0 1 3 4v3',
    staff: 'M12 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0 M3 21v-3a6 6 0 0 1 12 0v3m2-16a3 3 0 0 1 0 6m2 3a5 5 0 0 1 3 4v3',
    groups: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
    journals: 'M6 3h13v18H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3zM3 17h16M8 7h7m-7 4h5',
    statistics: 'M4 3v17h17M8 15v-4m5 4V6m5 9V9',
    clubs: 'M19 10c0 6-7 11-7 11S5 16 5 10a7 7 0 0 1 14 0z M14 10a2 2 0 1 1-4 0 2 2 0 0 1 4 0',
    history: 'M3 11a9 9 0 1 1 2 7M3 4v7h7m2-5v6l4 2',
    more: 'M5 12h.01M12 12h.01M19 12h.01',
  };
  return <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={section === 'more' ? 4 : 1.7} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"><path d={paths[section]} /></svg>;
}
