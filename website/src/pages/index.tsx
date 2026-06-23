import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

type TrackCard = {
  title: string;
  description: string;
  to: string;
  cta: string;
};

const tracks: TrackCard[] = [
  {
    title: 'Get started',
    description:
      'Create tasks, write acceptance criteria, and run execute-and-verify cycles with a dedicated worktree.',
    to: '/execute-and-verify',
    cta: 'Run your first task',
  },
  {
    title: 'Understand the system',
    description:
      'See how taskapi, Postgres, SSE, and the agent worker fit together — with diagrams and worked examples.',
    to: '/architecture',
    cta: 'Read architecture',
  },
  {
    title: 'Build on Hamix',
    description:
      'Contributor paths for the API, web UI, harness, and tests. Start from the guide, then dive into domain articles.',
    to: '/guide',
    cta: 'Open the guide',
  },
];

function TrackCardItem({title, description, to, cta}: TrackCard) {
  return (
    <div className={clsx('col col--4')}>
      <div className={styles.trackCard}>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
        <Link className="button button--primary button--block" to={to}>
          {cta}
        </Link>
      </div>
    </div>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Documentation"
      description="Hamix documentation — control plane for coding agents">
      <header className={clsx('hero hero--primary', styles.heroBanner)}>
        <div className="container">
          <img
            className={styles.wordmark}
            src={`${siteConfig.baseUrl}img/hamix-wordmark.png`}
            alt="Hamix"
          />
          <p className={styles.tagline}>{siteConfig.tagline}</p>
          <p className={styles.lead}>
            Coordinate Cursor CLI and other agentic systems with structured tasks,
            acceptance criteria, and independent verification.
          </p>
          <div className={styles.heroButtons}>
            <Link className="button button--primary button--lg" to="/guide">
              Start reading
            </Link>
            <Link
              className="button button--secondary button--lg"
              to="/api">
              API reference
            </Link>
          </div>
        </div>
      </header>
      <main className={styles.tracks}>
        <div className="container">
          <Heading as="h2">Choose a learning path</Heading>
          <p className={styles.tracksIntro}>
            Each track tells you what you will learn and what to read next. You do
            not need to read everything — pick the row that matches your goal.
          </p>
          <div className="row">
            {tracks.map((track) => (
              <TrackCardItem key={track.title} {...track} />
            ))}
          </div>
        </div>
      </main>
    </Layout>
  );
}
