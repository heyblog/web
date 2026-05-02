import { spawnSync } from "node:child_process";
import { appendFileSync } from "node:fs";

const zeroSha = /^0+$/;

const nodeModules = [
  { id: "configs", package: "@zhblogs/configs", path: "packages/configs" },
  { id: "utils", package: "@zhblogs/utils", path: "packages/utils" },
  { id: "db", package: "@zhblogs/db", path: "packages/db" },
  { id: "api", package: "@zhblogs/api", path: "apps/api" },
  { id: "web", package: "@zhblogs/web", path: "apps/web" },
  { id: "cloudflare", package: "@zhblogs/cloudflare", path: "apps/cloudflare" },
];

const goModules = [
  { id: "worker", package: "@zhblogs/worker", path: "apps/worker" },
  { id: "deployer", package: "@zhblogs/deployer", path: "apps/deployer" },
];

const dockerImages = [
  { id: "api", dockerfile: "apps/api/Dockerfile", image: "zhblogs-api" },
  { id: "web", dockerfile: "apps/web/Dockerfile", image: "zhblogs-web" },
  {
    id: "worker",
    dockerfile: "apps/worker/Dockerfile",
    image: "zhblogs-worker",
  },
];

const appToNode = new Map([
  ["api", "api"],
  ["web", "web"],
  ["cloudflare", "cloudflare"],
]);

const appToGo = new Map([
  ["worker", "worker"],
  ["deployer", "deployer"],
]);

function git(args) {
  const result = spawnSync("git", args, { encoding: "utf8" });
  if (result.status === 0) return result.stdout.trim();
  if (result.error) throw result.error;
  throw new Error(result.stderr.trim() || `git ${args.join(" ")} failed`);
}

function parseArgs(argv) {
  const args = new Map();
  for (let index = 0; index < argv.length; index += 2) {
    const key = argv[index];
    if (!key.startsWith("--") || index + 1 >= argv.length) {
      throw new Error(`invalid argument: ${key}`);
    }
    args.set(key.slice(2), argv[index + 1]);
  }
  return args;
}

function verifyCommit(ref) {
  if (!ref || zeroSha.test(ref)) return null;
  try {
    return git(["rev-parse", "--verify", `${ref}^{commit}`]);
  } catch {
    return null;
  }
}

function resolveBase(base, head) {
  const verifiedBase = verifyCommit(base);
  if (verifiedBase) return verifiedBase;
  const roots = git(["rev-list", "--max-parents=0", head]).split("\n");
  return roots.at(-1);
}

function listChangedFiles(base, head) {
  const output = git(["diff", "--name-only", base, head]);
  return output
    .split("\n")
    .map((file) => file.trim())
    .filter(Boolean);
}

function addAllCode(affected) {
  for (const module of nodeModules) affected.node.add(module.id);
  for (const module of goModules) affected.go.add(module.id);
}

function applyFileRule(affected, file) {
  if (file === ".env.example") {
    affected.needsEnvManualReview = true;
    affected.notifyDeployer = true;
    return;
  }

  if (file.startsWith("infra/")) {
    affected.needsInfraSync = true;
    affected.notifyDeployer = true;
    return;
  }

  if (file.startsWith("packages/")) {
    addAllCode(affected);
    affected.deployCloudflare = true;
    affected.notifyDeployer = true;
    if (file.startsWith("packages/db/")) affected.needsDbMigrate = true;
    return;
  }

  if (file.startsWith("apps/")) {
    applyAppRule(affected, file);
    return;
  }

  if (isCodeGlobalFile(file)) {
    addAllCode(affected);
    affected.deployCloudflare = true;
    affected.notifyDeployer = true;
  }
}

function applyAppRule(affected, file) {
  const [, app] = file.split("/");
  const nodeId = appToNode.get(app);
  if (nodeId) affected.node.add(nodeId);
  const goId = appToGo.get(app);
  if (goId) affected.go.add(goId);
  if (["api", "web", "worker"].includes(app)) affected.notifyDeployer = true;
  if (app === "cloudflare") affected.deployCloudflare = true;
  if (app === "deployer") {
    affected.needsDeployerUpdate = true;
    affected.notifyDeployer = true;
  }
}

function isCodeGlobalFile(file) {
  if (file.startsWith("scripts/") || file.startsWith(".githooks/")) return true;
  if (file.startsWith(".github/")) return true;
  return [
    ".dockerignore",
    ".editorconfig",
    ".nvmrc",
    "commitlint.config.ts",
    "package.json",
    "pnpm-lock.yaml",
    "pnpm-workspace.yaml",
    "renovate.json",
  ].includes(file);
}

function analyze(files) {
  const affected = {
    node: new Set(),
    go: new Set(),
    deployCloudflare: false,
    notifyDeployer: false,
    needsDbMigrate: false,
    needsInfraSync: false,
    needsDeployerUpdate: false,
    needsEnvManualReview: false,
  };
  for (const file of files) applyFileRule(affected, file);
  return affected;
}

function toMatrix(items) {
  return { include: items };
}

function selectedModules(modules, selected) {
  return modules.filter((module) => selected.has(module.id));
}

function buildOutputs(affected, files) {
  const node = selectedModules(nodeModules, affected.node);
  const go = selectedModules(goModules, affected.go);
  const docker = dockerImages.filter(
    (image) => affected.node.has(image.id) || affected.go.has(image.id),
  );
  const hasCode = node.length > 0 || go.length > 0;
  return {
    node_packages_json: JSON.stringify(toMatrix(node)),
    go_packages_json: JSON.stringify(toMatrix(go)),
    docker_images_json: JSON.stringify(toMatrix(docker)),
    has_node: String(node.length > 0),
    has_go: String(go.length > 0),
    has_code: String(hasCode),
    has_docker: String(docker.length > 0),
    deploy_cloudflare: String(affected.deployCloudflare),
    notify_deployer: String(affected.notifyDeployer),
    needs_db_migrate: String(affected.needsDbMigrate),
    needs_infra_sync: String(affected.needsInfraSync),
    needs_deployer_update: String(affected.needsDeployerUpdate),
    needs_env_manual_review: String(affected.needsEnvManualReview),
    changed_files: files.join("\n"),
  };
}

function writeGithubOutput(outputs) {
  if (!process.env.GITHUB_OUTPUT) return;
  const delimiter = `affected_modules_${Date.now()}`;
  let content = "";
  for (const [key, value] of Object.entries(outputs)) {
    content += value.includes("\n")
      ? `${key}<<${delimiter}\n${value}\n${delimiter}\n`
      : `${key}=${value}\n`;
  }
  appendFileSync(process.env.GITHUB_OUTPUT, content);
}

const args = parseArgs(process.argv.slice(2));
const head = verifyCommit(args.get("head") ?? "HEAD") ?? verifyCommit("HEAD");
if (!head) throw new Error("unable to resolve head commit");
const base = resolveBase(args.get("base"), head);
const changedFiles = listChangedFiles(base, head);
const affected = analyze(changedFiles);
const outputs = buildOutputs(affected, changedFiles);

writeGithubOutput(outputs);
console.log(
  JSON.stringify({ base, head, changed_files: changedFiles, outputs }, null, 2),
);
