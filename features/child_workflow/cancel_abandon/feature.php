<?php

declare(strict_types=1);

namespace Harness\Feature\ChildWorkflow\CancelAbandon;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Exception\Failure\ChildWorkflowFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class MainWorkflow
{
    #[WorkflowMethod('MainWorkflow')]
    public function run()
    {
        $child = Workflow::newUntypedChildWorkflowStub(
            'Harness_ChildWorkflow_CancelAbandon_Child',
            Workflow\ChildWorkflowOptions::new()
                ->withParentClosePolicy(Workflow\ParentClosePolicy::Abandon),
        );

        yield $child->start('test 42');

        try {
            return yield $child->getResult();
        } catch (CanceledFailure) {
            return 'cancelled';
        } catch (ChildWorkflowFailure $failure) {
            # Check CanceledFailure
            return $failure->getPrevious()::class === CanceledFailure::class
                ? 'child-cancelled'
                : throw $failure;
        }
    }
}

#[WorkflowInterface]
class ChildWorkflow
{
    private bool $exit = false;

    #[WorkflowMethod('Harness_ChildWorkflow_CancelAbandon_Child')]
    public function run(string $input)
    {
        yield Workflow::await(fn(): bool => $this->exit);
        return $input;
    }

    #[Workflow\SignalMethod('exit')]
    public function exit(): void
    {
        $this->exit = true;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('MainWorkflow')] WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
    ): void {
        # Find the child workflow execution ID
        $deadline = \microtime(true) + 10;
        child_id:
        $execution = null;
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            if ($event->hasChildWorkflowExecutionStartedEventAttributes()) {
                $execution = $event->getChildWorkflowExecutionStartedEventAttributes()->getWorkflowExecution();
                break;
            }
        }

        if ($execution === null && \microtime(true) < $deadline) {
            goto child_id;
        }

        Assert::notNull($execution, 'Child workflow execution not found in history');

        # Get Child Workflow Stub
        $child = $client->newUntypedRunningWorkflowStub(
            $execution->getWorkflowId(),
            $execution->getRunId(),
            'Harness_ChildWorkflow_CancelAbandon_Child',
        );

        # Cancel the parent workflow
        $stub->cancel();
        # Expect the CanceledFailure in the parent workflow
        Assert::same('cancelled', $stub->getResult());

        # Signal the child workflow to exit
        $child->signal('exit');
        # No canceled failure in the child workflow
        Assert::same('test 42', $child->getResult());
    }
}
