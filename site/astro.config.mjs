import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightBlog from 'starlight-blog';
import tailwindcss from '@tailwindcss/vite';
import { analyticsBeacon } from './src/analytics.mjs';
import { rehypeTableScroll } from './src/rehype-table-scroll.mjs';

const repo = 'https://github.com/SREsAreHumanToo/ARTEX-CC';
const site = 'https://artex-cc.com';

export default defineConfig({
  site,
  trailingSlash: 'always',
  markdown: { rehypePlugins: [rehypeTableScroll] },
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
      plugins: [
        starlightBlog({
          title: 'Blog',
          authors: {
            artex: { name: 'ARTEX-CC maintainers', url: repo },
          },
          metrics: { readingTime: true },
          postCount: 10,
          recentPostCount: 5,
        }),
      ],
      customCss: ['./src/styles/global.css', './src/styles/blog.css'],
      components: {
        ThemeProvider: './src/components/ThemeProvider.astro',
        Footer: './src/components/Footer.astro',
        MarkdownContent: './src/components/blog/MarkdownContent.astro',
      },
      head: [
        { tag: 'meta', attrs: { property: 'og:image', content: `${site}/social-preview.png` } },
        { tag: 'meta', attrs: { property: 'og:image:width', content: '1280' } },
        { tag: 'meta', attrs: { property: 'og:image:height', content: '640' } },
        { tag: 'meta', attrs: { name: 'twitter:image', content: `${site}/social-preview.png` } },
        { tag: 'script', attrs: analyticsBeacon },
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
            'docs/plugins',
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
            { label: 'Blog', link: '/blog/' },
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
