import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import type { Plugin } from 'vite';

// Only build outputs go into the allowlist. Runtime responses never extend it.
export function pwaBuild(): Plugin {
  return {
    name: 'pride-pwa', apply: 'build', enforce: 'post',
    generateBundle(_, bundle) {
      const html = bundle['index.html'];
      if (!html || html.type !== 'asset') throw new Error('PWA requires index.html');
      const manifestSource = readFileSync(new URL('./public/manifest.webmanifest', import.meta.url), 'utf8');
      const manifest = JSON.parse(manifestSource) as { icons: { src: string }[] };
      const template = readFileSync(new URL('./service-worker.js', import.meta.url), 'utf8');
      const assets = Object.keys(bundle).filter((path) => path.startsWith('assets/'));
      const version = createHash('sha256').update(JSON.stringify(assets)).update(String(html.source)).update(template).update(manifestSource).digest('hex').slice(0, 16);
      const shell = `/assets/pwa-shell-${version}.html`;
      this.emitFile({ type: 'asset', fileName: shell.slice(1), source: html.source });
      const files = [...assets.map((path) => `/${path}`), ...manifest.icons.map((icon) => icon.src), shell];
      const worker = template
        .replace('__CACHE_NAME__', JSON.stringify(`pride-shell-${version}`))
        .replace('__STATIC_FILES__', JSON.stringify(files))
        .replace('__SHELL_PATH__', JSON.stringify(shell));
      this.emitFile({ type: 'asset', fileName: 'sw.js', source: worker });
    },
  };
}
