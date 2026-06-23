import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import {remarkRewriteHamixLinks} from './src/remark/rewriteHamixLinks';

const config: Config = {
  title: 'Hamix',
  tagline: 'Control plane for coding agents',
  favicon: 'img/hamix-wordmark.png',

  future: {
    v4: true,
  },

  url: 'https://alexsanderhamir.github.io',
  baseUrl: '/Hamix/',
  trailingSlash: false,

  organizationName: 'AlexsanderHamir',
  projectName: 'Hamix',

  onBrokenLinks: 'throw',

  markdown: {
    mermaid: true,
    format: 'md',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  themes: [
    '@docusaurus/theme-mermaid',
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      {
        hashed: true,
        docsRouteBasePath: ['/', 'contributing'],
        indexPages: true,
        highlightSearchTermsOnTargetPage: true,
      },
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/AlexsanderHamir/Hamix/tree/main/',
          beforeDefaultRemarkPlugins: [remarkRewriteHamixLinks],
          exclude: ['**/plans/**', 'README.md'],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'contributing',
        path: '..',
        routeBasePath: 'contributing',
        sidebarPath: './sidebarsContributing.ts',
        include: ['CONTRIBUTING.md', 'AGENTS.md'],
        beforeDefaultRemarkPlugins: [remarkRewriteHamixLinks],
        editUrl: 'https://github.com/AlexsanderHamir/Hamix/tree/main/',
      },
    ],
  ],

  themeConfig: {
    image: 'img/hamix-wordmark.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Hamix',
      logo: {
        alt: 'Hamix',
        src: 'img/hamix-wordmark.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'hamixSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'docSidebar',
          sidebarId: 'contributingSidebar',
          docsPluginId: 'contributing',
          position: 'left',
          label: 'Contributing',
        },
        {
          href: 'https://github.com/AlexsanderHamir/Hamix',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Learn',
          items: [
            {label: 'Get started', to: '/execute-and-verify'},
            {label: 'Architecture', to: '/architecture'},
            {label: 'API reference', to: '/api'},
          ],
        },
        {
          title: 'Contribute',
          items: [
            {label: 'Setup', to: '/contributing/CONTRIBUTING'},
            {label: 'Agent map', to: '/agent-map'},
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/AlexsanderHamir/Hamix',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Hamix. MIT License.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
    mermaid: {
      theme: {light: 'neutral', dark: 'dark'},
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
