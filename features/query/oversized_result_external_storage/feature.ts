import * as assert from 'assert';
import { ExternalStorage, type Payload, type StorageDriver, StorageDriverClaim } from '@temporalio/common';
import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

// Exceed the server limit so the SDK must offload the result.
const RESULT_SIZE = 3 * 1024 * 1024;
const STORAGE_THRESHOLD = 1024;

const query = wf.defineQuery<string>('oversized-result');
const finishSignal = wf.defineSignal('finish');

const payloads = new Map<string, Payload>();
let stores = 0;
let retrieves = 0;

const driver: StorageDriver = {
  name: 'query-result-memory',
  type: 'query-result-memory',
  async store(_context, values) {
    stores++;
    return values.map((value) => {
      const key = `payload-${payloads.size}`;
      payloads.set(key, value);
      return new StorageDriverClaim({ key });
    });
  },
  async retrieve(_context, claims) {
    retrieves++;
    return claims.map((claim) => {
      const key = claim.claimData.key;
      const value = payloads.get(key);
      if (value === undefined) {
        throw new Error(`Payload ${key} not found`);
      }
      return value;
    });
  },
};

const externalStorage = new ExternalStorage({
  drivers: [driver],
  payloadSizeThreshold: STORAGE_THRESHOLD,
});

export async function workflow(): Promise<void> {
  wf.setHandler(query, () => 'a'.repeat(RESULT_SIZE));
  await new Promise<void>((resolve) => wf.setHandler(finishSignal, resolve));
}

export const feature = new Feature({
  workflow,
  dataConverter: { externalStorage },
  checkResult: async (runner, handle) => {
    const result = await handle.query(query);
    assert.equal(result, 'a'.repeat(RESULT_SIZE));
    assert.ok(stores > 0);
    assert.ok(retrieves > 0);

    await handle.signal(finishSignal);
    await runner.waitForRunResult(handle);
  },
});
