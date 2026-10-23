import { writeFileSync, rmSync } from 'node:fs';

// Only used with the smoke runner's isolated, local Terraform backend. Retain
// ownership across a failed import, so the outer finally block can destroy it.
export function importWithRecovery(terraform, address, checkpointFile) {
  const checkpoint = terraform('state', 'pull');
  writeFileSync(checkpointFile, checkpoint, { mode: 0o600 });
  try {
    terraform('state', 'rm', address);
    terraform('import', '-input=false', address, 'workspace');
  } catch (error) {
    // state rm/import can advance the local serial without changing remote
    // settings. Force restores the known pre-import bindings in this private
    // consumer only; no shared/remote backend is ever configured here.
    terraform('state', 'push', '-force', checkpointFile);
    throw error;
  } finally {
    rmSync(checkpointFile, { force: true });
  }
}
