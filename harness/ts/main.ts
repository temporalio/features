import { FeatureSource, Runner } from './harness';
import { Runtime, DefaultLogger } from '@temporalio/worker';
import pkg from '@temporalio/worker/lib/pkg';
import { Command } from 'commander';
import * as path from 'path';
import { TLSConfig } from '@temporalio/client';
import * as fs from 'fs';

async function run() {
  const program = new Command();
  program
    .requiredOption('--server <address>', 'The host:port of the server')
    .requiredOption('--namespace <namespace>', 'The namespace to use')
    .option('--client-cert-path <clientCertPath>', 'Path to a client certificate for TLS')
    .option('--client-key-path <clientKeyPath>', 'Path to a client key for TLS')
    .option('--proxy-control-uri <uri>', 'Base URL for simulating network outages via temporal-features-test-proxy')
    .argument('<features...>', 'Features as dir + ":" + task queue');

  const opts = program.parse(process.argv).opts<{
    server: string;
    namespace: string;
    clientCertPath: string;
    clientKeyPath: string;
    proxyControlUri: string;
    featureAndTaskQueues: string[];
  }>();
  opts.featureAndTaskQueues = program.args;

  console.log('Running TypeScript SDK version ' + pkg.version, 'against', opts.server);

  // Set logging to debug
  Runtime.install({ logger: new DefaultLogger('DEBUG') });

  // Collect all feature sources
  const featureRootDir = path.join(__dirname, '../../features');
  const sources = await FeatureSource.findAll(featureRootDir);

  // Load TLS certs if specified
  let tlsConfig: TLSConfig | undefined;
  if (opts.clientCertPath) {
    if (!opts.clientKeyPath) {
      throw new Error('Client cert path specified but no key path!');
    }
    const crt = fs.readFileSync(opts.clientCertPath);
    const key = fs.readFileSync(opts.clientKeyPath);
    tlsConfig = {};
    tlsConfig.clientCertPair = {
      crt,
      key,
    };
  } else if (opts.clientKeyPath && !opts.clientCertPath) {
    throw new Error('Client key path specified but no cert path!');
  }

  // Run each
  // TODO(cretz): Concurrent with log capturing
  let failureCount = 0;
  let failedFeaturesStr = '';
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
        tlsConfig,
      });
      await runner.run();
    } catch (err) {
      const errstr = `Feature ${featureDir} failed with ${err}`;
      failedFeaturesStr += errstr + '\n';
      console.error(errstr, (err as any).stack);
      failureCount++;
    } finally {
      await runner?.close();
    }
  }

  if (failureCount > 0) {
    throw new Error(`${failureCount} feature(s) failed: \n${failedFeaturesStr}`);
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
