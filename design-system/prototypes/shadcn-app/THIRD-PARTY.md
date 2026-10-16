# Сторонние ресурсы

- Оригинальная эмблема PRIDE и Rubik импортируются из общего `design-system/`; происхождение и лицензии — [manifest](../../manifest.json), [Rubik OFL](../../fonts/OFL-Rubik.txt). Ресурсы не перерабатывались.
- `src/components/ui/`, `src/hooks/use-mobile.ts`, `src/lib/utils.ts` установлены официальным shadcn CLI 4.21.5 из radix-nova. License MIT: [SHADCN-LICENSE.md](SHADCN-LICENSE.md). Локальные изменения: русские подписи Sheet/Sidebar, SidebarInset как div (один main), success/warning Badge.
- `src/styles/shadcn.css` — неизменённые utilities/variants `shadcn@4.21.5/dist/tailwind.css` с добавленной строкой происхождения. Они сохранены локально, чтобы приложение не зависело от дерева установщика CLI. npm audit выявил 7 high в транзитивном дереве CLI (braces/micromatch/fast-glob/ts-morph); удаление ненужной runtime-зависимости CLI устранило их. CLI для последующего добавления доступен через npx, в runtime не входит.
- React, Radix, Lucide, Vite, Tailwind и остальные зависимости зафиксированы `package-lock.json`; их лицензии сохраняются в npm packages. Значки — Lucide, без самодельной иллюстрации или иконочного шрифта.
