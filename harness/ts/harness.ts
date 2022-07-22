import { Connection, WorkflowClient, WorkflowHandleWithFirstExecutionRunId } from '@temporalio/client';
import { ActivityInterface, Workflow, WorkflowResultType } from '@temporalio/common';
import { Worker, WorkerOptions, NativeConnection } from '@temporalio/worker';
import { promises as fs } from 'fs';
import * as path from 'path';
import { v4 as uuidv4 } from 'uuid';

export interface FeatureOptions<W extends Workflow, A extends ActivityInterface> {
  /**
   * Workflow to execute. This defaults to `import(workflowsPath).workflow` if
   * unset.
   */
  workflow?: W;

  /**
   * Activities to register if any.
   */
  activities?: A;

  /**
   * Path to for WorkerOptions.workflowsPath. Defaults to the feature directory
   * + '/feature.workflow.ts'.
   */
  workflowsPath?: string;

  /**
   * Execute the workflow. If unset, defaults to
   * Runner.executeSingleParameterlessWorkflow.
   */
  execute?: (runner: Runner<W, A>) => Promise<WorkflowHandleWithFirstExecutionRunId<W>>;

  /**
   * Wait on and check the result of the workflow. If unset, defaults to
   * Runner.waitForRunResult.
   */
  checkResult?: (runner: Runner<W, A>, handle: WorkflowHandleWithFirstExecutionRunId<W>) => Promise<void>;

  /**
   * Check the history of the workflow run. TODO(cretz): Unhandled currently
   */
  checkHistory?: (runner: Runner<W, A>, handle: WorkflowHandleWithFirstExecutionRunId<W>) => Promise<void>;
}

export class Feature<W extends Workflow, A extends ActivityInterface> {
  constructor(readonly options: FeatureOptions<W, A>) {}

  public get activities(): A | ActivityInterface {
    return this.options.activities ?? {};
  }
}

export class FeatureSource {
  static async findAll(absDir: string, origRootDir: string = absDir): Promise<FeatureSource[]> {
    const dirents = await fs.readdir(absDir, { withFileTypes: true });
    const dirs = [];
    for (const dirent of dirents) {
      if (dirent.name === 'feature.js') {
        const relDir = path.relative(origRootDir, absDir).replaceAll(path.sep, '/');
        dirs.push(new FeatureSource(relDir, absDir));
      } else if (dirent.isDirectory()) {
        dirs.push(...(await FeatureSource.findAll(path.join(absDir, dirent.name), origRootDir)));
      }
    }
    return dirs;
  }

  constructor(
    // Relative to features/ root _and_ uses / for platform independence
    readonly relDir: string,
    readonly absDir: string
  ) {}

  loadFeature<W extends Workflow, A extends ActivityInterface>(): Feature<W, A> {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    return require(path.join(this.absDir, 'feature.js')).feature;
  }
}

export interface RunnerOptions {
  address: string;
  namespace: string;
  taskQueue: string;
}

export class Runner<W extends Workflow, A extends ActivityInterface> {
  static async create(source: FeatureSource, options: RunnerOptions): Promise<Runner<Workflow, ActivityInterface>> {
    // Load the feature
    const feature = source.loadFeature();

    // Connect to client
    const connection = await Connection.connect({
      address: options.address,
    });
    const client = new WorkflowClient({
      connection,
      namespace: options.namespace,
    });

    // Create a connection for the Worker
    const nativeConn = await NativeConnection.create({
      address: options.address,
    });

    // Create and start the worker
    const workflowsPath =
      feature.options.workflowsPath ?? require.resolve(path.join(source.absDir, 'feature.workflow.js'));
    const workerOpts: WorkerOptions = {
      connection: nativeConn,
      namespace: options.namespace,
      workflowsPath,
      activities: feature.activities,
      taskQueue: options.taskQueue,
    };
    const worker = await Worker.create(workerOpts);
    const workerRunPromise = (async () => {
      try {
        await worker.run();
      } finally {
        await connection.close();
      }
    })();

    return new Runner(source, feature, workflowsPath, options, client, nativeConn, worker, workerRunPromise);
  }

  private constructor(
    readonly source: FeatureSource,
    readonly feature: Feature<W, A>,
    readonly workflowsPath: string,
    readonly options: RunnerOptions,
    readonly client: WorkflowClient,
    readonly nativeConnection: NativeConnection,
    readonly worker: Worker,
    readonly workerRunPromise: Promise<void>
  ) {}

  async run(): Promise<void> {
    // Run the workflow and fail if workflow or worker fails
    return await Promise.race([this.workerRunPromise, this.runWorkflow()]);
  }

  private async runWorkflow() {
    // Start
    console.log(`Executing feature ${this.source.relDir}`);
    let handle;
    if (this.feature.options.execute) {
      handle = await this.feature.options.execute(this);
    } else {
      handle = await this.executeSingleParameterlessWorkflow();
    }

    // Result check
    console.log(`Checking result on feature ${this.source.relDir}`);
    if (this.feature.options.checkResult) {
      console.log('Using custom result checker');
      await this.feature.options.checkResult(this, handle);
    } else {
      await this.waitForRunResult(handle);
    }

    // History check
    // TODO(cretz): This
    if (this.feature.options.checkHistory) {
      await this.feature.options.checkHistory(this, handle);
    }
  }

  async executeSingleParameterlessWorkflow(): Promise<WorkflowHandleWithFirstExecutionRunId> {
    const workflow = this.feature.options.workflow ?? 'workflow';
    return await this.client.start<() => any>(workflow, {
      taskQueue: this.options.taskQueue,
      workflowId: this.source.relDir + '-' + uuidv4(),
      workflowExecutionTimeout: 60000,
    });
  }

  async waitForRunResult<W extends Workflow>(
    run: WorkflowHandleWithFirstExecutionRunId<W>
  ): Promise<WorkflowResultType<W>> {
    return await run.result();
  }

  async close(): Promise<void> {
    this.worker.shutdown();
    await this.workerRunPromise;
    await this.nativeConnection.close();
  }
}
