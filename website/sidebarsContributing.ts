import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  contributingSidebar: [
    {
      type: 'doc',
      id: 'CONTRIBUTING',
      label: 'Setup and PR checklist',
    },
    {
      type: 'doc',
      id: 'AGENTS',
      label: 'Code paths (agents)',
    },
  ],
};

export default sidebars;
