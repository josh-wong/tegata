// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  tutorialSidebar: [
    {
      type: 'doc',
      id: 'introduction',
    },
    {
      type: 'category',
      label: 'Getting started',
      link: {
        type: 'generated-index',
        description: 'Learn what Tegata is and get your vault set up.',
      },
      items: ['quickstart', 'os-setup'],
    },
    {
      type: 'category',
      label: 'Guides',
      link: {
        type: 'generated-index',
        description: 'Walkthroughs for the desktop GUI, terminal UI, and CLI.',
      },
      items: ['gui-guide', 'tui-guide', 'cli-reference'],
    },
    {
      type: 'category',
      label: 'Audit logging',
      link: {
        type: 'generated-index',
        description: 'Enable and verify the optional tamper-evident audit log.',
      },
      items: ['scalardl-setup'],
    },
    {
      type: 'category',
      label: 'Reference',
      link: {
        type: 'generated-index',
        description: 'Security best practices, troubleshooting, and frequently asked questions.',
      },
      items: ['security-best-practices', 'troubleshooting', 'faq', 'privacy-and-disclaimer'],
    },
  ],

  releaseNotesSidebar: [
    {
      type: 'category',
      label: 'Release notes',
      link: {
        type: 'generated-index',
      },
      items: ['release-notes-v1'],
    },
  ],

  supportSidebar: [
    {
      type: 'category',
      label: 'Support the project',
      link: {
        type: 'generated-index',
      },
      items: [
        {
          type: 'link',
          label: 'GitHub Sponsors',
          href: 'https://github.com/sponsors/josh-wong',
        },
        {
          type: 'link',
          label: 'Ko-fi',
          href: 'https://ko-fi.com/josh_haha',
        },
        {
          type: 'link',
          label: 'PayPal',
          href: 'https://www.paypal.me/joshww',
        },
      ],
    },
  ],
};

export default sidebars;
