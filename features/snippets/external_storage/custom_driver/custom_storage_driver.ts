import { randomUUID } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import {
  StorageDriverClaim,
  type Payload,
  type StorageDriver,
  type StorageDriverRetrieveContext,
  type StorageDriverStoreContext,
} from '@temporalio/common';
import { temporal } from '@temporalio/proto';

const PayloadProto = temporal.api.common.v1.Payload;

// @@@SNIPSTART typescript-custom-storage-driver
export class LocalDiskStorageDriver implements StorageDriver {
  readonly name = 'my-local-disk';
  readonly type = 'local-disk';

  constructor(private readonly storeDir = '/tmp/temporal-payload-store') {}

  async store(context: StorageDriverStoreContext, payloads: Payload[]): Promise<StorageDriverClaim[]> {
    let dir = this.storeDir;
    const target = context.target;
    if (target?.id) {
      // `target.kind` is either 'workflow' or 'activity'. Including it in the path keeps a
      // Workflow Id from colliding with an Activity Id in the same Namespace.
      dir = path.join(this.storeDir, target.namespace, target.kind, target.id);
    }
    await mkdir(dir, { recursive: true });

    const claims: StorageDriverClaim[] = [];
    for (const payload of payloads) {
      const filePath = path.join(dir, `${randomUUID()}.bin`);
      await writeFile(filePath, PayloadProto.encode(payload).finish());
      claims.push(new StorageDriverClaim({ path: filePath }));
    }
    return claims;
  }

  async retrieve(_context: StorageDriverRetrieveContext, claims: StorageDriverClaim[]): Promise<Payload[]> {
    const payloads: Payload[] = [];
    for (const claim of claims) {
      const filePath = claim.claimData.path;
      if (!filePath) {
        throw new Error("claim is missing required 'path' data");
      }
      payloads.push(PayloadProto.decode(await readFile(filePath)));
    }
    return payloads;
  }
}
// @@@SNIPEND
