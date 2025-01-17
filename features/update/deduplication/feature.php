<?php

declare(strict_types=1);

namespace Harness\Feature\Update\Deduplication;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\Update\LifecycleStage;
use Temporal\Client\Update\UpdateOptions;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $counter = 0;
    private bool $blocked = true;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->counter >= 2);
        yield Workflow::await(fn(): bool => Workflow::allHandlersFinished());
        return $this->counter;
    }

    #[Workflow\SignalMethod('unblock')]
    public function unblock()
    {
        $this->blocked = false;
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate()
    {
        ++$this->counter;
        # Verify that dedupe works pre-update-completion
        yield Workflow::await(fn(): bool => !$this->blocked);
        $this->blocked = true;
        return $this->counter;
    }
}

class FeatureChecker
{
    #[Check]
    public function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
    ): void {
        $updateId = 'incrementer';
        # Issue async update

        $handle1 = $stub->startUpdate(
            UpdateOptions::new('my_update', LifecycleStage::StageAccepted)
                ->withUpdateId($updateId),
        );
        $handle2 = $stub->startUpdate(
            UpdateOptions::new('my_update', LifecycleStage::StageAccepted)
                ->withUpdateId($updateId),
        );

        $stub->signal('unblock');

        Assert::same($handle1->getResult(1), 1);
        Assert::same($handle2->getResult(1), 1);

        # This only needs to start to unblock the workflow
        $stub->startUpdate('my_update');

        # There should be two accepted updates, and only one of them should be completed with the set id
        $totalUpdates = 0;
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            $event->hasWorkflowExecutionUpdateAcceptedEventAttributes() and ++$totalUpdates;

            $f = $event->getWorkflowExecutionUpdateCompletedEventAttributes();
            $f === null or Assert::same($f->getMeta()?->getUpdateId(), $updateId);
        }

        Assert::same($totalUpdates, 2);
    }
}
