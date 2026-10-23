import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, rmSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const project = fileURLToPath(new URL("../", import.meta.url));
const terraform = fileURLToPath(new URL("../../terraform-provider-codetelemetry/", import.meta.url));
const binary = fileURLToPath(new URL("../bin/terraform-provider-codetelemetry", import.meta.url));
mkdirSync(new URL("../bin/", import.meta.url), { recursive: true });

// Build the shared implementation; never copy or reimplement its API behavior.
execFileSync(process.env.GO_BINARY || "go", [
  "build", `-ldflags=-X main.version=${process.env.PROVIDER_VERSION || "0.1.0"}`, "-o", binary, ".",
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
  // Keep generated npm metadata in the project namespace, including local SDKs.
  const manifest = new URL("package.json", sdk);
  const metadata = JSON.parse(readFileSync(manifest, "utf8"));
  metadata.name = "@codetelemetry/pulumi-codetelemetry";
  metadata.license = "MPL-2.0";
  writeFileSync(manifest, JSON.stringify(metadata, null, 2) + "\n");
  cpSync(new URL("../../LICENSE", import.meta.url), new URL("LICENSE", sdk));
} finally {
  rmSync(scratch, { recursive: true, force: true });
}

// Install from the committed lockfile. `package add` runs npm install, which can
// rewrite local-package metadata differently between npm major versions.
execFileSync(process.platform === "win32" ? "npm.cmd" : "npm", ["ci"], {
  cwd: project, stdio: "inherit", timeout: 300_000,
});
