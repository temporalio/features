import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test, { type TestContext } from 'node:test';
import {
  latestSemver,
  parseSemver,
  sdkPackages,
  updateSdk,
  type Manifest,
  type NpmRunner,
} from './update-temporal-sdk.mts';

const manifest = {
  dependencies: {
    '@temporalio/common': '1.26.0',
    '@temporalio/interceptors-opentelemetry': '1.26.0',
    'nexus-rpc': '^0.0.3',
  },
};

function fixture(t: TestContext): string {
  const directory = mkdtempSync(join(tmpdir(), 'sdk-updater-test-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(join(directory, 'package.json'), JSON.stringify(manifest));
  writeFileSync(join(directory, 'package-lock.json'), '{}');
  return directory;
}

await test('discovers OTel interceptors but excludes independently versioned dependencies', () => {
  assert.deepEqual(sdkPackages(manifest), ['@temporalio/common', '@temporalio/interceptors-opentelemetry']);
});

await test('parses stable semver components and rejects non-exact or prerelease inputs', () => {
  assert.deepEqual(parseSemver('1.10.12'), { version: '1.10.12', major: 1, minor: 10, patch: 12 });
  for (const input of ['^1.27.0', '1.27.0-rc.1', '01.27.0', '1.27', null, 127]) {
    assert.equal(parseSemver(input), undefined);
  }
});

await test('selects the latest parsed semver numerically', () => {
  const versions = ['1.9.0', '1.10.2', '1.10.12', '1.27.0-rc.1'].map(parseSemver).filter((v) => v !== undefined);
  assert.equal(latestSemver(versions).version, '1.10.12');
  assert.equal(latestSemver(versions.slice(0, 1)).version, '1.9.0');
  assert.throws(() => latestSemver([]), /No stable/);
});

await test('selects within the current major and resolves before exact pinning', (t) => {
  const directory = fixture(t);
  const calls: string[][] = [];
  const runNpm: NpmRunner = (args, cwd) => {
    calls.push(args);
    if (args[0] === 'view') {
      assert.equal(args[1], '@temporalio/common@1');
      return '["1.26.0","1.27.0"]';
    }
    assert.notEqual(cwd, directory);
    const staged = JSON.parse(readFileSync(join(cwd, 'package.json'), 'utf8')) as Manifest;
    assert.equal(staged.dependencies['@temporalio/common'], '^1.26.0');
    assert.equal(staged.dependencies['@temporalio/interceptors-opentelemetry'], '^1.26.0');
    if (args[0] === 'install') {
      assert.equal(calls.at(-2)?.[0], 'update');
      for (const name of sdkPackages(staged)) {
        assert.ok(args.includes(`${name}@1.27.0`));
        staged.dependencies[name] = '1.27.0';
      }
      writeFileSync(join(cwd, 'package.json'), JSON.stringify(staged));
      writeFileSync(join(cwd, 'package-lock.json'), '{"updated":true}');
    }
    return '';
  };
  assert.equal(updateSdk({ directory, runNpm }), '1.27.0');
  assert.deepEqual(
    calls.map((args) => args[0]),
    ['view', 'update', 'install'],
  );
  const updated = JSON.parse(readFileSync(join(directory, 'package.json'), 'utf8')) as Manifest;
  assert.equal(updated.dependencies['@temporalio/interceptors-opentelemetry'], '1.27.0');
  assert.equal(updated.dependencies['nexus-rpc'], '^0.0.3');
  assert.equal(readFileSync(join(directory, 'package-lock.json'), 'utf8'), '{"updated":true}');
});

for (const failure of ['view', 'update', 'install']) {
  await test(`a failed ${failure} leaves both project files untouched`, (t) => {
    const directory = fixture(t);
    const before = ['package.json', 'package-lock.json'].map((file) => readFileSync(join(directory, file), 'utf8'));
    const runNpm: NpmRunner = (args) => {
      if (args[0] === failure) throw new Error('simulated failure');
      return '"1.27.0"';
    };
    assert.throws(() => updateSdk({ directory, runNpm }), /simulated failure/);
    assert.deepEqual(
      ['package.json', 'package-lock.json'].map((file) => readFileSync(join(directory, file), 'utf8')),
      before,
    );
  });
}

await test('rejects ranges and prereleases as explicit versions', (t) => {
  const directory = fixture(t);
  for (const version of ['^1.27.0', '1.27.0-rc.1']) {
    assert.throws(() => updateSdk({ directory, version }), /exact stable/);
  }
});
