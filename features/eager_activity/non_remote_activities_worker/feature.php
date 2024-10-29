<?php

declare(strict_types=1);

namespace Harness\Feature\EagerActivity\NonRemoteActivitiesWorker;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Harness\Exception\SkipTest;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowFailedException;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::newActivityStub(
            EmptyActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(3),
        )->dummy();
    }
}

/**
 * Not a local activity
 */
#[ActivityInterface]
class EmptyActivity
{
    #[ActivityMethod('dummy')]
    public function dummy(): void
    {
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub
    ): void {
        throw new SkipTest('Need to run worker with no_remote_activities=True');

        try {
            $stub->getResult();
        } catch (WorkflowFailedException $e) {
            // todo check that previous exception is a timeout_error and not a schedule_to_start_error
        }

        throw new \Exception('Test not completed');
    }
}
