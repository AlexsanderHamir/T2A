import type {Root} from 'mdast';
import {visit} from 'unist-util-visit';
import path from 'node:path';
import fs from 'node:fs';

const REPO_ROOT = path.resolve(__dirname, '../../..');
const GITHUB_BLOB = 'https://github.com/AlexsanderHamir/Hamix/blob/main';

function isDocsMarkdown(rel: string): boolean {
  return rel.startsWith('docs/') && rel.endsWith('.md');
}

function docsRouteFromRel(rel: string): string {
  return `/${rel.slice('docs/'.length).replace(/\.md$/, '')}`;
}

/** Rewrites markdown links so Docusaurus can resolve and route them. */
export function remarkRewriteHamixLinks() {
  return async (tree: Root, file: {path?: string}) => {
    if (!file?.path) {
      return;
    }
    const docDir = path.dirname(file.path);

    visit(tree, 'link', (node) => {
      const url = node.url;
      if (!url || url.startsWith('http') || url.startsWith('#') || url.startsWith('mailto:')) {
        return;
      }

      const [rawPath, hash] = url.split('#');
      const hashSuffix = hash ? `#${hash}` : '';

      if (
        rawPath === '../CONTRIBUTING.md' ||
        rawPath === '../../CONTRIBUTING.md' ||
        rawPath.endsWith('/CONTRIBUTING.md')
      ) {
        node.url = `/contributing/CONTRIBUTING${hashSuffix}`;
        return;
      }
      if (
        rawPath === '../AGENTS.md' ||
        rawPath === '../../AGENTS.md' ||
        rawPath.endsWith('/AGENTS.md')
      ) {
        node.url = `/contributing/AGENTS${hashSuffix}`;
        return;
      }
      if (rawPath === '../README.md' || rawPath === '../../README.md') {
        node.url = `/${hashSuffix}`;
        return;
      }
      if (rawPath === 'README.md' || rawPath === './README.md') {
        const inDocsFolder = file.path.replace(/\\/g, '/').includes('/docs/');
        node.url = inDocsFolder ? `/guide${hashSuffix}` : `/${hashSuffix}`;
        return;
      }
      if (rawPath === 'docs/README.md' || rawPath.endsWith('/docs/README.md')) {
        node.url = `/guide${hashSuffix}`;
        return;
      }

      const resolved = path.normalize(path.resolve(docDir, rawPath));

      if (!resolved.startsWith(REPO_ROOT)) {
        return;
      }

      const rel = path.relative(REPO_ROOT, resolved).replace(/\\/g, '/');

      if (isDocsMarkdown(rel)) {
        if (rel === 'docs/README.md') {
          node.url = `/guide${hashSuffix}`;
          return;
        }
        const route = docsRouteFromRel(rel);
        node.url = `${route}${hashSuffix}`;
        return;
      }

      if (fs.existsSync(resolved)) {
        node.url = `${GITHUB_BLOB}/${rel}${hashSuffix}`;
      } else if (rawPath.endsWith('.md')) {
        // Relative doc link without a resolvable target — strip .md for Docusaurus IDs.
        node.url = `${rawPath.replace(/\.md$/, '')}${hashSuffix}`;
      }
    });
  };
}
