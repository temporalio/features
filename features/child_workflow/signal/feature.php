<?php

declare(strict_types=1);

namespace Harness\Feature\ChildWorkflow\Signal;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use React\Promise\PromiseInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

/**
 * A Workflow that starts a Child Workflow, unblocks it, and returns the result of the child workflow.
 */
#[WorkflowInterface]
class MainWorkflow
{
    #[WorkflowMethod('MainWorkflow')]
    public function run()
    {
        $workflow = Workflow::newChildWorkflowStub(
            ChildWorkflow::class,
            // TODO: remove after https://github.com/temporalio/sdk-php/issues/451 is fixed
            Workflow\ChildWorkflowOptions::new()->withTaskQueue(Workflow::getInfo()->taskQueue),
        );
        $handle = $workflow->run();
        yield $workflow->signal('unblock');
        return yield $handle;
    }
}

/**
 * A workflow that waits for a signal and returns the data received.
 */
#[WorkflowInterface]
class ChildWorkflow
{
    private ?string $message = null;

    #[WorkflowMethod('ChildWorkflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->message !== null);
        return $this->message;
    }

    /**
     * @return PromiseInterface<null>
     */
    #[SignalMethod('signal')]
    public function signal(string $message): void
    {
        $this->message = $message;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(#[Stub('MainWorkflow')] WorkflowStubInterface $stub): void
    {
        Assert::same($stub->getResult(), 'unblock');
    }
}
