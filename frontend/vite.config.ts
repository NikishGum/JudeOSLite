import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath } from 'node:url';
import { pwaBuild } from './pwa-build.ts';

export default defineConfig({
  plugins: [tailwindcss(), react(), pwaBuild()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: { rolldownOptions: { output: { codeSplitting: { groups: [{ name: 'ui-vendor', test: /node_modules\/(?:@radix-ui|radix-ui|lucide-react|class-variance-authority|clsx|cn|tailwind-merge)\// }] } } } },
  server: {
    host: '127.0.0.1', port: 5173, strictPort: true,
    fs: { allow: ['.', '../design-system'] },
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
});
