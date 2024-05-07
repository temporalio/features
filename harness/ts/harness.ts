import { randomUUID } from 'node:crypto';
import {
  Connection,
  Client,
  WorkflowHandleWithFirstExecutionRunId,
  WorkflowHandle,
  WorkflowStartOptions,
  TLSConfig,
  ConnectionOptions,
} from '@temporalio/client';
import * as proto from '@temporalio/proto';
import { DataConverter, UntypedActivities, Workflow, WorkflowResultType } from '@temporalio/common';
import { Worker, WorkerOptions, NativeConnection, NativeConnectionOptions } from '@temporalio/worker';
import { promises as fs } from 'fs';
import * as path from 'path';
import { ConnectionInjectorInterceptor } from './activity-interceptors';
import { setTimeout } from 'timers/promises';
export { getConnection, getClient, Context } from './activity-interceptors';

export interface FeatureOptions<W extends Workflow, A extends UntypedActivities> {
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
   * Optional workflow start options for default execute. Some values are
   * defaulted if unset (e.g. task queue and workflow execution timeout).
   */
  workflowStartOptions?: Partial<WorkflowStartOptions<W>>;

  /**
   * Optional worker options to augment worker creation for the feature.
   */
  workerOptions?: Partial<WorkerOptions>;

  /**
   * Override the default data converter
   */
  dataConverter?: DataConverter;

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

  /**
   * If set, use this instead of the default run function which races the
   * worker run & workflow run.
   */
  alternateRun?: (runner: Runner<W, A>) => Promise<void>;
}

export class Feature<W extends Workflow, A extends UntypedActivities> {
  constructor(readonly options: FeatureOptions<W, A>) {}

  public get activities(): A | UntypedActivities {
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

  loadFeature<W extends Workflow, A extends UntypedActivities>(): Feature<W, A> {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    return require(path.join(this.absDir, 'feature.js')).feature;
  }
}

export interface RunnerOptions {
  address: string;
  namespace: string;
  proxyUrl?: string;
  taskQueue: string;
  tlsConfig?: TLSConfig;
}

export class Runner<W extends Workflow, A extends UntypedActivities> {
  static async create(source: FeatureSource, options: RunnerOptions): Promise<Runner<Workflow, UntypedActivities>> {
    // Load the feature
    const feature = source.loadFeature();

    // Connect to client
    const connectionOpts: ConnectionOptions = {
      address: options.address,
      tls: options.tlsConfig,
    };
    const connection = await Connection.connect(connectionOpts);
    const client = new Client({
      connection,
      namespace: options.namespace,
      dataConverter: feature.options.dataConverter,
    });

    // Create a connection for the Worker
    const nativeConnectionOpts: NativeConnectionOptions = {
      address: options.address,
      tls: options.tlsConfig,
    };
    const nativeConn = await NativeConnection.connect(nativeConnectionOpts);

    // Create and start the worker
    const workflowsPath = feature.options.workflowsPath ?? require.resolve(path.join(source.absDir, 'feature.js'));
    const workerOpts: WorkerOptions = {
      connection: nativeConn,
      namespace: options.namespace,
      workflowsPath,
      activities: feature.activities,
      taskQueue: options.taskQueue,
      dataConverter: feature.options.dataConverter,
      bundlerOptions: {
        webpackConfigHook(config) {
          return {
            ...config,
            // Map these "unsafe" modules to global variables that will be injected by the interceptor below
            externals: {
              '@temporalio/harness': 'temporalioHarness',
              '@temporalio/client': 'temporalioClient',
              '@temporalio/activity': 'temporalioActivity',
            },
          };
        },
      },
      interceptors: {
        activityInbound: [() => new ConnectionInjectorInterceptor(connection, client)],
        workflowModules: [require.resolve('./workflow-globals-injection-interceptors')],
      },
      ...feature.options.workerOptions,
    };
    const worker = await Worker.create(workerOpts);
    const workerRunPromise = (async () => {
      await worker.run();
    })();
    return new Runner(
      source,
      feature,
      options,
      client,
      connectionOpts,
      nativeConn,
      nativeConnectionOpts,
      worker,
      workerOpts,
      workerRunPromise
    );
  }

  private constructor(
    readonly source: FeatureSource,
    readonly feature: Feature<W, A>,
    readonly options: RunnerOptions,
    readonly client: Client,
    readonly connectionOpts: ConnectionOptions,
    readonly nativeConnection: NativeConnection,
    readonly nativeConnectionOpts: NativeConnectionOptions,
    private _worker: Worker,
    readonly workerOpts: WorkerOptions,
    private _workerRunPromise: Promise<void>
  ) {}

  async run(): Promise<void> {
    if (this.feature.options.alternateRun) {
      return await this.feature.options.alternateRun(this);
    } else {
      // Run the workflow and fail if workflow or worker fails
      return await Promise.race([this._workerRunPromise, this.runWorkflow()]);
    }
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
    await this.checkWorkflowResults(handle);
  }

  /**
   * Performs checks for the workflow result / history with overrides if specified in the feature.
   * You don't need to call this unless you're overriding run.
   */
  async checkWorkflowResults(handle: WorkflowHandleWithFirstExecutionRunId): Promise<void> {
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

  get worker(): Worker {
    return this._worker;
  }

  get workerRunPromise(): Promise<void> {
    return this._workerRunPromise;
  }

  async restartWorker(): Promise<void> {
    this._worker = await Worker.create(this.workerOpts);
    this._workerRunPromise = (async () => {
      await this._worker.run();
    })();
  }

  async executeSingleParameterlessWorkflow(): Promise<WorkflowHandleWithFirstExecutionRunId> {
    return this.executeParameterlessWorkflow(this.feature.options.workflow ?? 'workflow');
  }

  async executeParameterlessWorkflow(workflow: W | 'workflow'): Promise<WorkflowHandleWithFirstExecutionRunId> {
    const startOptions: WorkflowStartOptions = {
      taskQueue: this.options.taskQueue,
      workflowId: `${this.source.relDir}-${randomUUID()}`,
      workflowExecutionTimeout: 60000,
      ...(this.feature.options.workflowStartOptions ?? {}),
    };
    return await this.client.workflow.start(workflow, startOptions);
  }

  async waitForRunResult<W extends Workflow>(
    run: WorkflowHandleWithFirstExecutionRunId<W>
  ): Promise<WorkflowResultType<W>> {
    return await run.result();
  }

  async close(): Promise<void> {
    this._worker.shutdown();
    try {
      await this._workerRunPromise;
    } finally {
      await this.client.connection.close();
      await this.nativeConnection.close();
    }
  }

  async getHistoryEvents(handle: WorkflowHandle): Promise<proto.temporal.api.history.v1.IHistoryEvent[]> {
    let nextPageToken: Uint8Array | undefined = undefined;
    const history = Array<proto.temporal.api.history.v1.IHistoryEvent>();
    for (;;) {
      const response: proto.temporal.api.workflowservice.v1.GetWorkflowExecutionHistoryResponse =
        await this.client.connection.workflowService.getWorkflowExecutionHistory({
          nextPageToken,
          namespace: this.options.namespace,
          execution: { workflowId: handle.workflowId },
        });
      history.push(...(response.history?.events ?? []));
      if (response.nextPageToken == null || response.nextPageToken.length === 0) break;
      nextPageToken = response.nextPageToken;
    }
    return history;
  }

  async getWorkflowResultPayload(handle: WorkflowHandle): Promise<proto.temporal.api.common.v1.IPayload | void> {
    const events = await this.getHistoryEvents(handle);
    const completedEvent = events.find(
      ({ workflowExecutionCompletedEventAttributes }) => !!workflowExecutionCompletedEventAttributes
    );
    return completedEvent?.workflowExecutionCompletedEventAttributes?.result?.payloads?.[0];
  }

  async getWorkflowArgumentPayload(handle: WorkflowHandle): Promise<proto.temporal.api.common.v1.IPayload | void> {
    const events = await this.getHistoryEvents(handle);
    const startedEvent = events.find(
      ({ workflowExecutionStartedEventAttributes }) => !!workflowExecutionStartedEventAttributes
    );
    return startedEvent?.workflowExecutionStartedEventAttributes?.input?.payloads?.[0];
  }
}

export async function retry(fn: () => Promise<boolean>, retries = 3, duration = 1000): Promise<boolean> {
  for (let i = 0; i < retries; i++) {
    if (await fn()) {
      return true;
    }
    await setTimeout(duration);
  }
  return false;
}
