<?php

declare(strict_types=1);

namespace Harness\Feature\Update\WorkerRestart;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Harness\Runtime\Runner;
use Psr\Container\ContainerInterface;
use Spiral\RoadRunner\KeyValue\StorageInterface;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

const KV_ACTIVITY_STARTED = 'update-worker-restart-started';
const KV_ACTIVITY_BLOCKED = 'update-worker-restart-blocked';

#[WorkflowInterface]
class FeatureWorkflow
{
    private bool $done = false;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->done);

        return 'Hello, World!';
    }

    #[Workflow\UpdateMethod('do_activities')]
    public function doActivities()
    {
        yield Workflow::executeActivity(
            'blocks',
            options: ActivityOptions::new()->withStartToCloseTimeout(10)
        );
        $this->done = true;
    }
}

#[ActivityInterface]
class FeatureActivity
{
    public function __construct(
        private StorageInterface $kv,
    ) {}

    #[ActivityMethod('blocks')]
    public function blocks(): string
    {
        $this->kv->set(KV_ACTIVITY_STARTED, true);

        do {
            $blocked = $this->kv->get(KV_ACTIVITY_BLOCKED, true);

            if (!$blocked) {
                break;
            }

            \usleep(100_000);
        } while (true);

        return 'hi';
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
        ContainerInterface $c,
        Runner $runner,
    ): void {
        $handle = $stub->startUpdate('do_activities');

        # Wait for the activity to start.
        $deadline = \microtime(true) + 20;
        do {
            if ($c->get(StorageInterface::class)->get(KV_ACTIVITY_STARTED, false)) {
                break;
            }

            \microtime(true) > $deadline and throw throw new \RuntimeException('Activity did not start');
            \usleep(100_000);
        } while (true);

        # Restart the worker.
        $runner->stop();
        $runner->start();
        # Unblocks the activity.
        $c->get(StorageInterface::class)->set(KV_ACTIVITY_BLOCKED, false);

        # Wait for Temporal restarts the activity
        $handle->getResult(30);
        $stub->getResult();
    }
}
