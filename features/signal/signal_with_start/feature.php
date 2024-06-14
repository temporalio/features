<?php

declare(strict_types=1);

namespace Harness\Feature\Signal\SignalWithStart;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $value = 0;

    #[WorkflowMethod('Workflow')]
    public function run(): int
    {
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
    public static function check(
        WorkflowClientInterface $client,
        Feature $feature,
    ): void {
        $stub = $client->newWorkflowStub(
            FeatureWorkflow::class,
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue),
        );
        $run = $client->startWithSignal($stub, 'add', [42]);
        // See https://github.com/temporalio/sdk-php/issues/457
        Assert::same($run->getResult(), 42, 'Signal must be executed before Workflow handler');
    }
}
