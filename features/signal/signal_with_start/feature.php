<?php

declare(strict_types=1);

namespace Harness\Feature\Signal\SignalWithStart;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Harness\Runtime\Feature;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $value = 0;

    #[WorkflowMethod('Workflow')]
    public function run(int $arg = 0)
    {
        $this->value += $arg;

        yield Workflow::await(fn() => $this->value > 0);

        return $this->value;
    }

    #[SignalMethod('add')]
    public function add(int $arg): void
    {
        $this->value += $arg;
    }
}

class FeatureChecker
{
    #[Check]
    public static function checkSignalProcessedBeforeHandler(
        WorkflowClientInterface $client,
        Feature $feature,
    ): void {
        $stub = $client->newWorkflowStub(
            FeatureWorkflow::class,
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue),
        );
        $run = $client->startWithSignal($stub, 'add', [42], [1]);

        // See https://github.com/temporalio/sdk-php/issues/457
        Assert::same($run->getResult(), 43, 'Signal must be processed before WF handler. Result: ' . $run->getResult());
    }

    #[Check]
    public static function checkSignalToExistingWorkflow(
        #[Stub('Workflow', args: [-2])] WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        Feature $feature,
    ): void {
        $stub2 = $client->newWorkflowStub(
            FeatureWorkflow::class,
            WorkflowOptions::new()
                ->withTaskQueue($feature->taskQueue)
                // Reuse same ID
                ->withWorkflowId($stub->getExecution()->getID()),
        );
        $run = $client->startWithSignal($stub2, 'add', [42]);

        Assert::same($run->getResult(), 40, 'Existing WF must be reused. Result: ' . $run->getResult());
    }
}
