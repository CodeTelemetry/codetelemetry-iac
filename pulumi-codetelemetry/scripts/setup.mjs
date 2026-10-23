import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const project = fileURLToPath(new URL("../", import.meta.url));
const terraform = fileURLToPath(new URL("../../terraform-provider-codetelemetry/", import.meta.url));
const binary = fileURLToPath(new URL("../bin/terraform-provider-codetelemetry", import.meta.url));
mkdirSync(new URL("../bin/", import.meta.url), { recursive: true });

// Build the shared implementation; never copy or reimplement its API behavior.
execFileSync(process.env.GO_BINARY || "go", [
  "build", "-ldflags=-X main.version=0.1.0", "-o", binary, ".",
], { cwd: terraform, stdio: "inherit", timeout: 300_000 });

// A relative path keeps the project portable; regenerate the SDK on each host.
// SDK generation needs no cloud account or persistent stack backend.
const scratch = mkdtempSync(join(tmpdir(), "codetelemetry-pulumi-setup-"));
try {
  execFileSync(process.env.PULUMI_BINARY || "pulumi", [
    "package", "gen-sdk", "terraform-provider@1.5.0",
    "--language", "nodejs", "--local", "--out", scratch,
    "--non-interactive", "--", "./bin/terraform-provider-codetelemetry",
  ], {
    cwd: project, stdio: "inherit", timeout: 300_000,
    env: { ...process.env, PULUMI_BACKEND_URL: pathToFileURL(scratch).href },
  });
  const sdk = new URL("../sdks/codetelemetry/", import.meta.url);
  rmSync(sdk, { recursive: true, force: true });
  cpSync(join(scratch, "nodejs"), fileURLToPath(sdk), { recursive: true });
} finally {
  rmSync(scratch, { recursive: true, force: true });
}

// Install from the committed lockfile. `package add` runs npm install, which can
// rewrite local-package metadata differently between npm major versions.
execFileSync(process.platform === "win32" ? "npm.cmd" : "npm", ["ci"], {
  cwd: project, stdio: "inherit", timeout: 300_000,
});
