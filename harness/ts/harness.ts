import { Connection, WorkflowClient, WorkflowHandleWithRunId } from '@temporalio/client';
import { ActivityInterface, Workflow } from '@temporalio/common';
import { Worker } from '@temporalio/worker';
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
  execute?: (runner: Runner<W, A>) => Promise<WorkflowHandleWithRunId<W>>;

  /**
   * Wait on and check the result of the workflow. If unset, defaults to
   * Runner.waitForRunResult.
   */
  checkResult?: (runner: Runner<W, A>, handle: WorkflowHandleWithRunId<W>) => Promise<void>;

  /**
   * Check the history of the workflow run. TODO(cretz): Unhandled currently
   */
  checkHistory?: (runner: Runner<W, A>, handle: WorkflowHandleWithRunId<W>) => Promise<void>;
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
      if (dirent.name === 'feature.ts') {
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
    return require(path.join(this.absDir, 'feature.ts')).feature;
  }
}

export interface RunnerOptions {
  address: string;
  namespace: string;
  taskQueue: string;
}

export class Runner<W extends Workflow, A extends ActivityInterface> {
  static async create(source: FeatureSource, options: RunnerOptions) {
    // Load the feature
    const feature = source.loadFeature();

    // Connect to client
    const conn = new Connection({
      address: options.address,
    });
    const client = new WorkflowClient(conn.service, {
      namespace: options.namespace,
    });

    // Create and start the worker
    const workflowsPath =
      feature.options.workflowsPath ?? require.resolve(path.join(source.absDir, 'feature.workflow.ts'));
    const worker = await Worker.create({
      workflowsPath,
      activities: feature.activities,
      taskQueue: options.taskQueue,
    });
    const workerRunPromise = worker.run().finally(() => conn.client.close());

    return new Runner(source, feature, workflowsPath, options, conn, client, worker, workerRunPromise);
  }

  private constructor(
    readonly source: FeatureSource,
    readonly feature: Feature<W, A>,
    readonly workflowsPath: string,
    readonly options: RunnerOptions,
    readonly conn: Connection,
    readonly client: WorkflowClient,
    readonly worker: Worker,
    readonly workerRunPromise: Promise<void>
  ) {}

  async run() {
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

  async executeSingleParameterlessWorkflow() {
    const workflow = this.feature.options.workflow ?? require(this.workflowsPath).workflow;
    return await this.client.start<() => any>(workflow, {
      taskQueue: this.options.taskQueue,
      workflowId: this.source.relDir + '-' + uuidv4(),
    });
  }

  async waitForRunResult<W extends Workflow>(run: WorkflowHandleWithRunId<W>) {
    return await run.result();
  }

  close() {
    this.worker.shutdown();
  }
}
