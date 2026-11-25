import { readFile } from 'node:fs/promises';
import { defineCollection } from 'astro:content';
import type { Loader } from 'astro/loaders';
import { docsLoader } from '@astrojs/starlight/loaders';
import { docsSchema } from '@astrojs/starlight/schema';
import { blogSchema } from 'starlight-blog/schema';

// Renders Markdown files that live at the repository root (and must stay there for
// GitHub) without copying them. The leading H1 is dropped because the page supplies its own.
function repoMarkdown(files: Record<string, string>): Loader {
  return {
    name: 'repo-markdown',
    async load({ config, store, renderMarkdown }) {
      for (const [id, file] of Object.entries(files)) {
        const url = new URL(`../${file}`, config.root);
        const body = (await readFile(url, 'utf8')).replace(/^# .*\n+/, '');
        store.set({ id, data: { file }, body, rendered: await renderMarkdown(body) });
      }
    },
  };
}

export const collections = {
  docs: defineCollection({ loader: docsLoader(), schema: docsSchema({ extend: (context) => blogSchema(context) }) }),
  repo: defineCollection({
    loader: repoMarkdown({ changelog: 'CHANGELOG.md', security: 'SECURITY.md' }),
  }),
};
