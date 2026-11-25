import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

const repo = 'https://github.com/SREsAreHumanToo/ARTEX-CC';
const site = 'https://artex-cc.com';

export default defineConfig({
  site,
  trailingSlash: 'always',
  integrations: [
    starlight({
      title: 'ARTEX-CC',
      description:
        'Use your Claude subscription to run pentests. No API key needed. An MCP server that gives Claude Code an asset graph, findings, coverage, traffic capture and reports.',
      logo: {
        light: '../docs/brand/logo-light.svg',
        dark: '../docs/brand/logo-dark.svg',
        replacesTitle: true,
      },
      favicon: '/favicon.svg',
      social: [{ icon: 'github', label: 'GitHub', href: repo }],
      editLink: { baseUrl: `${repo}/edit/main/site/` },
      lastUpdated: true,
      customCss: ['./src/styles/global.css'],
      components: {
        ThemeProvider: './src/components/ThemeProvider.astro',
      },
      head: [
        { tag: 'meta', attrs: { property: 'og:image', content: `${site}/social-preview.png` } },
        { tag: 'meta', attrs: { property: 'og:image:width', content: '1280' } },
        { tag: 'meta', attrs: { property: 'og:image:height', content: '640' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: `${site}/social-preview.png` } },
      ],
      sidebar: [
        {
          label: 'Guides',
          items: [
            'docs/getting-started',
            'docs/mcp-clients',
            'docs/configuration',
            'docs/web-ui',
            'docs/hands-free',
            'docs/walkthrough',
          ],
        },
        {
          label: 'Tool reference',
          items: [{ autogenerate: { directory: 'docs/tools' } }],
        },
        {
          label: 'Internals',
          items: ['docs/architecture', 'docs/troubleshooting'],
        },
        {
          label: 'Evals',
          items: [{ autogenerate: { directory: 'evals' } }],
        },
        {
          label: 'Project',
          items: [
            { label: 'Changelog', link: '/changelog/' },
            'roadmap',
            { label: 'Security', link: '/security/' },
            { label: 'Live demo', link: '/demo/' },
          ],
        },
      ],
    }),
  ],
  vite: {
    plugins: [tailwindcss()],
    server: { fs: { allow: ['..'] } },
  },
});
