// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// There are various equivalent ways to declare your Docusaurus config.
// See: https://docusaurus.io/docs/api/docusaurus-config

import {themes as prismThemes} from 'prism-react-renderer';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Tegata Documentation',
  tagline: 'Your authentication history. Integrity checked.',
  favicon: 'img/favicon.svg',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  markdown: {
    mdx1Compat: {
      headingIds: true,
    },
  },

  // Set the production url of your site here
  url: 'https://tegata.080f53.com',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/',

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: 'josh-wong', // Usually your GitHub org/user name.
  projectName: 'tegata', // Usually your repo name.

  onBrokenLinks: 'throw',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'ja'],
    localeConfigs: {
      en: {
        label: 'English',
        direction: 'ltr',
        htmlLang: 'en',
      },
      ja: {
        label: '日本語',
        direction: 'ltr',
        htmlLang: 'ja',
      },
    },
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: './sidebars.js',
          // Please change this to your repo.
          // Remove this to remove the "edit this page" links.
          editUrl:
            'https://github.com/josh-wong/tegata/tree/main/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      // Replace with your project's social card
      image: 'img/tegata-social-card.jpg',
      colorMode: {
        respectPrefersColorScheme: true,
      },
      docs: {
        sidebar: {
          hideable: true,
          autoCollapseCategories: true,
        },
      },
      navbar: {
        title: 'Tegata',
        logo: {
          alt: 'Tegata logo',
          src: 'img/logo.svg',
        },
        items: [
          /* {
            type: 'docSidebar',
            sidebarId: 'tutorialSidebar',
            position: 'left',
            label: 'Docs',
          }, */
          {
            type: 'docSidebar',
            sidebarId: 'tutorialSidebar',
            position: 'left',
            label: 'Docs',
          },
          {
            type: 'docSidebar',
            sidebarId: 'releaseNotesSidebar',
            position: 'left',
            label: 'Release notes',
          },
          {
            href: 'https://github.com/josh-wong/tegata',
            label: 'GitHub',
            position: 'right',
          },
          {
            type: 'localeDropdown',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        /* links: [
          {
            title: 'Docs',
            items: [
              {
                label: 'Quickstart',
                to: '/docs/quickstart',
              },
              {
                label: 'CLI reference',
                to: '/docs/cli-reference',
              },
              {
                label: 'Security best practices',
                to: '/docs/security-best-practices',
              },
            ],
          },
          {
            title: 'More',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/josh-wong/tegata',
              },
              {
                label: 'Releases',
                href: 'https://github.com/josh-wong/tegata/releases',
              },
            ],
          },
        ], */
        copyright: `Copyright © ${new Date().getFullYear()} Josh Wong. Built with Docusaurus.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
      },
      algolia: {
        // The application ID provided by Algolia
        appId: '6G0F38QD9K',

        // Public API key: it is safe to commit it
        apiKey: 'ef6970ac3647aaa2db175c91770fca7e',

        indexName: 'Tegata Documentation',

        // Optional: see doc section below
        contextualSearch: true,

        // Optional: Algolia search parameters
        searchParameters: {},

        // Optional: path for search page that enabled by default (`false` to disable it)
        searchPagePath: 'search',

        // Optional: whether the insights feature is enabled or not on Docsearch (`false` by default)
        insights: true,

        // Optional: whether you want to use the new Ask AI feature (undefined by default)
        // askAi: 'YOUR_ALGOLIA_ASK_AI_ASSISTANT_ID',
      },
    }),
};

export default config;
