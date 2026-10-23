import { resolve } from 'node:path';
import { rmSync } from 'node:fs';
import { consumer } from './consumer.mjs';
const project = consumer(resolve(process.argv[2]), process.argv[3]);
rmSync(project.root, { recursive: true, force: true });
console.log('Fresh Terraform filesystem-mirror and Pulumi npm-tarball installations passed.');
