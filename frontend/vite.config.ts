import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { pwaBuild } from './pwa-build.ts';

export default defineConfig({
  plugins: [react(), pwaBuild()],
  server: {
    host: '127.0.0.1', port: 5173, strictPort: true,
    fs: { allow: ['.', '../design-system'] },
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
});
