import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { spawn } from "node:child_process";

const DEFAULT_NODE_COUNT = 200_000;
const DEFAULT_BRANCHING_FACTOR = 4;
const ENV_FILE = ".env.local";
const ENV_KEY = "VITE_TASK_GRAPH_MOCK_URL";

function readArg(name) {
  const prefixed = `--${name}=`;
  const match = process.argv.find((entry) => entry.startsWith(prefixed));
  return match ? match.slice(prefixed.length) : undefined;
}

function parsePositiveInt(value, label) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    throw new Error(`Invalid ${label}: expected positive integer, got "${value}"`);
  }
  return parsed;
}

function formatSizeLabel(nodeCount) {
  if (nodeCount % 1_000_000 === 0) return `${nodeCount / 1_000_000}m`;
  if (nodeCount % 1_000 === 0) return `${nodeCount / 1_000}k`;
  return String(nodeCount);
}

function resolveNodeCount() {
  const sizeArg = readArg("size");
  if (sizeArg) {
    const value = sizeArg.trim().toLowerCase();
    if (/^\d+$/.test(value)) return parsePositiveInt(value, "--size");
    if (/^\d+k$/.test(value)) return parsePositiveInt(value.slice(0, -1), "--size") * 1_000;
    if (/^\d+m$/.test(value)) return parsePositiveInt(value.slice(0, -1), "--size") * 1_000_000;
    throw new Error(`Invalid --size value "${sizeArg}" (examples: 100k, 200k, 1m, 350000)`);
  }
  const nodeCountArg = readArg("node-count");
  if (nodeCountArg) return parsePositiveInt(nodeCountArg, "--node-count");
  return DEFAULT_NODE_COUNT;
}

async function generateMock(nodeCount, branchingFactor, output) {
  const scriptPath = resolve(process.cwd(), "scripts/generate-graph-mock.mjs");
  const args = [
    scriptPath,
    `--node-count=${nodeCount}`,
    `--branching-factor=${branchingFactor}`,
    `--output=${output}`,
  ];
  await new Promise((resolveDone, rejectDone) => {
    const child = spawn(process.execPath, args, { stdio: "inherit", cwd: process.cwd() });
    child.on("error", rejectDone);
    child.on("exit", (code) => {
      if (code === 0) {
        resolveDone();
        return;
      }
      rejectDone(new Error(`Graph mock generation failed with exit code ${String(code)}`));
    });
  });
}

async function upsertEnvLocal(mockUrl) {
  const envPath = resolve(process.cwd(), ENV_FILE);
  let current = "";
  try {
    current = await readFile(envPath, "utf8");
  } catch {
    current = "";
  }
  const lines = current === "" ? [] : current.split(/\r?\n/);
  const updatedLines = [];
  let replaced = false;
  for (const line of lines) {
    if (line.startsWith(`${ENV_KEY}=`)) {
      updatedLines.push(`${ENV_KEY}=${mockUrl}`);
      replaced = true;
      continue;
    }
    updatedLines.push(line);
  }
  if (!replaced) {
    if (updatedLines.length > 0 && updatedLines[updatedLines.length - 1] !== "") {
      updatedLines.push("");
    }
    updatedLines.push(`${ENV_KEY}=${mockUrl}`);
  }
  const next = `${updatedLines.join("\n").replace(/\n+$/, "")}\n`;
  await writeFile(envPath, next, "utf8");
}

async function main() {
  const nodeCount = resolveNodeCount();
  const branchingFactorArg = readArg("branching-factor");
  const branchingFactor = branchingFactorArg
    ? parsePositiveInt(branchingFactorArg, "--branching-factor")
    : DEFAULT_BRANCHING_FACTOR;

  const sizeLabel = formatSizeLabel(nodeCount);
  const output = `public/mock-data/graphs/task-graph-${sizeLabel}.json`;
  const mockUrl = `/${output.replace(/^public\//, "")}`;

  await generateMock(nodeCount, branchingFactor, output);
  await upsertEnvLocal(mockUrl);

  process.stdout.write(`Updated ${ENV_FILE}: ${ENV_KEY}=${mockUrl}\n`);
  process.stdout.write("Restart Vite dev server to apply env changes.\n");
}

main().catch((error) => {
  process.stderr.write(`${String(error)}\n`);
  process.exitCode = 1;
});
