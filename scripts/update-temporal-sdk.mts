import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export interface Manifest {
  dependencies: Record<string, string>;
  [key: string]: unknown;
}

export type NpmRunner = (args: string[], cwd: string) => string;

interface UpdateOptions {
  directory: string;
  version?: string;
  runNpm?: NpmRunner;
}

function npm(args: string[], cwd: string): string {
  return execFileSync('npm', args, { cwd, encoding: 'utf8' });
}

export function sdkPackages(manifest: Manifest): string[] {
  return Object.keys(manifest.dependencies).filter((name) => name.startsWith('@temporalio/'));
}

export interface SemVer {
  version: string;
  major: number;
  minor: number;
  patch: number;
}

// Only stable, exact releases are candidates for an SDK update.
export function parseSemver(value: unknown): SemVer | undefined {
  if (typeof value !== 'string') return undefined;
  const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.exec(value);
  if (!match) return undefined;
  return { version: value, major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) };
}

export function latestSemver(versions: SemVer[]): SemVer {
  const latest = versions.toSorted((a, b) => a.major - b.major || a.minor - b.minor || a.patch - b.patch).at(-1);
  if (!latest) throw new Error('No stable SDK release found');
  return latest;
}

export function updateSdk({ directory, version, runNpm = npm }: UpdateOptions): string {
  const manifest = JSON.parse(readFileSync(join(directory, 'package.json'), 'utf8')) as Manifest;
  const packages = sdkPackages(manifest);
  const current = parseSemver(manifest.dependencies['@temporalio/common']);
  if (!current) throw new Error('@temporalio/common must have an exact stable version');
  for (const name of packages) {
    if (!parseSemver(manifest.dependencies[name])) {
      throw new Error(`${name} must have an exact stable version`);
    }
  }
  let selected: SemVer;
  if (version === undefined) {
    const published: unknown = JSON.parse(
      runNpm(['view', `@temporalio/common@${current.major}`, 'version', '--json'], directory),
    );
    const candidates: unknown[] = Array.isArray(published) ? published : [published];
    selected = latestSemver(candidates.map(parseSemver).filter((v) => v !== undefined));
  } else {
    const parsed = parseSemver(version);
    if (!parsed) throw new Error('Expected an exact stable SDK version');
    selected = parsed;
  }

  const staging = mkdtempSync(join(tmpdir(), 'features-sdk-update-'));
  try {
    // Expand the pins so npm can update the peer-dependency group together
    // before pinning it to the selected release.
    for (const name of packages) manifest.dependencies[name] = `^${manifest.dependencies[name]}`;
    writeFileSync(join(staging, 'package.json'), `${JSON.stringify(manifest, null, 2)}\n`);
    writeFileSync(join(staging, 'package-lock.json'), readFileSync(join(directory, 'package-lock.json')));
    runNpm(['update', '--package-lock-only', '--ignore-scripts', ...packages], staging);
    runNpm(
      [
        'install',
        '--package-lock-only',
        '--ignore-scripts',
        '--save-exact',
        ...packages.map((name) => `${name}@${selected.version}`),
      ],
      staging,
    );
    // Resolution errors propagate and exit nonzero before either file is replaced.
    for (const file of ['package.json', 'package-lock.json']) {
      writeFileSync(join(directory, file), readFileSync(join(staging, file)));
    }
  } finally {
    rmSync(staging, { recursive: true, force: true });
  }
  return selected.version;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  if (process.argv.length > 3) throw new Error('Usage: npm run update:temporal-sdk -- [version]');
  console.log(`Updated Temporal SDK to ${updateSdk({ directory: process.cwd(), version: process.argv[2] })}`);
}
