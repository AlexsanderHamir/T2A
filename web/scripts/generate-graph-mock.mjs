import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

const VALID_STATUSES = ["ready", "running", "blocked", "review", "done", "failed"];
const VALID_PRIORITIES = ["low", "medium", "high", "critical"];

const DEFAULT_NODE_COUNT = 200_000;
const DEFAULT_BRANCHING_FACTOR = 4;
const DEFAULT_OUTPUT = "public/mock-data/graphs/task-graph-200k.json";

function readPositiveIntEnv(name, fallback) {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`Invalid ${name}: expected positive integer, got "${raw}"`);
  }
  return parsed;
}

function parsePositiveInt(value, label) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`Invalid ${label}: expected positive integer, got "${value}"`);
  }
  return parsed;
}

function readArg(name) {
  const prefixed = `--${name}=`;
  const match = process.argv.find((entry) => entry.startsWith(prefixed));
  return match ? match.slice(prefixed.length) : undefined;
}

function createNode(index) {
  const id = `mock-node-${index}`;
  const status = VALID_STATUSES[index % VALID_STATUSES.length];
  const priority = VALID_PRIORITIES[index % VALID_PRIORITIES.length];
  const title = `Mock graph node ${index.toLocaleString()}`;

  if (index === 0) {
    return {
      id,
      title,
      status,
      priority,
      children: [],
    };
  }

  return { id, title, status, priority, children: [] };
}

function linkChildren(nodes, branchingFactor) {
  for (let i = 1; i < nodes.length; i += 1) {
    const parentIndex = Math.floor((i - 1) / branchingFactor);
    nodes[parentIndex].children.push(nodes[i]);
  }
}

async function main() {
  const nodeCountArg = readArg("node-count");
  const branchingArg = readArg("branching-factor");
  const outputArg = readArg("output");

  const nodeCount =
    nodeCountArg !== undefined
      ? parsePositiveInt(nodeCountArg, "--node-count")
      : readPositiveIntEnv("GRAPH_MOCK_NODE_COUNT", DEFAULT_NODE_COUNT);
  const branchingFactor =
    branchingArg !== undefined
      ? parsePositiveInt(branchingArg, "--branching-factor")
      : readPositiveIntEnv("GRAPH_MOCK_BRANCHING_FACTOR", DEFAULT_BRANCHING_FACTOR);
  const output = outputArg ?? process.env.GRAPH_MOCK_OUTPUT ?? DEFAULT_OUTPUT;

  const nodes = new Array(nodeCount);
  for (let i = 0; i < nodeCount; i += 1) {
    nodes[i] = createNode(i);
  }
  linkChildren(nodes, branchingFactor);

  const root = nodes[0];
  const resolvedOutput = resolve(process.cwd(), output);
  const outputDir = dirname(resolvedOutput);
  await mkdir(outputDir, { recursive: true });
  await writeFile(resolvedOutput, JSON.stringify(root));

  process.stdout.write(
    `Generated ${nodeCount.toLocaleString()} nodes at ${resolvedOutput} (branching=${branchingFactor})\n`,
  );
}

main().catch((error) => {
  process.stderr.write(`${String(error)}\n`);
  process.exitCode = 1;
});
