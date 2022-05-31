import { FeatureSource, Runner } from './harness';
import { Runtime, DefaultLogger } from '@temporalio/worker';
import pkg from '@temporalio/worker/lib/pkg';
import { Command } from 'commander';
import * as path from 'path';

async function run() {
  const program = new Command();
  program
    .requiredOption('--server <address>', 'The host:port of the server')
    .requiredOption('--namespace <namespace>', 'The namespace to use')
    .argument('<features...>', 'Features as dir + ":" + task queue');

  const opts = program.parse(process.argv).opts<{
    server: string;
    namespace: string;
    featureAndTaskQueues: string[];
  }>();
  opts.featureAndTaskQueues = program.args;

  console.log('Running TypeScript SDK version ' + pkg.version, 'against', opts.server);

  // Install core with our address and namespace
  const logger = new DefaultLogger('DEBUG');
  Runtime.install({
    logger,
    telemetryOptions: { logForwardingLevel: 'OFF' },
  });

  // Collect all feature sources
  const featureRootDir = path.join(__dirname, '../../features');
  const sources = await FeatureSource.findAll(featureRootDir);

  // Run each
  // TODO(cretz): Concurrent with log capturing
  let failureCount = 0;
  for (const featureAndTaskQueue of opts.featureAndTaskQueues) {
    const [featureDir, taskQueueFromOpt] = featureAndTaskQueue.split(':');
    const taskQueue = taskQueueFromOpt ?? featureDir;

    let runner;
    try {
      // Find the source
      const source = sources.find((s) => s.relDir === featureDir);
      if (!source) {
        // noinspection ExceptionCaughtLocallyJS
        throw new Error(`feature ${featureDir} not found`);
      }

      // Run
      console.debug(`Running feature ${featureDir}`);
      runner = await Runner.create(source, {
        address: opts.server,
        namespace: opts.namespace,
        taskQueue,
      });
      await runner.run();
    } catch (err) {
      console.error(`Feature ${featureDir} failed with ${err}`, (err as any).stack);
      failureCount++;
    } finally {
      await runner?.close();
    }
  }

  if (failureCount > 0) {
    throw new Error(`${failureCount} feature(s) failed`);
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
