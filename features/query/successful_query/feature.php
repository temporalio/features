<?php

declare(strict_types=1);

namespace Harness\Feature\Query\SuccessfulQuery;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Workflow;
use Temporal\Workflow\QueryMethod;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $counter = 0;
    private bool $beDone = false;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->beDone);
    }

    #[QueryMethod('get_counter')]
    public function getCounter(): int
    {
        return $this->counter;
    }

    #[SignalMethod('inc_counter')]
    public function incCounter(): void
    {
        ++$this->counter;
    }

    #[SignalMethod('finish')]
    public function finish(): void
    {
        $this->beDone = true;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(WorkflowClientInterface $client, Feature $feature): void
    {
        $stub = $client->newWorkflowStub(
            FeatureWorkflow::class,
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue),
        );
        $run = $client->start($stub);

        \assert($stub->getCounter() === 0);

        $stub->incCounter();
        \assert($stub->getCounter() === 1);

        $stub->incCounter();
        $stub->incCounter();
        $stub->incCounter();
        \assert($stub->getCounter() === 4);

        $stub->finish();
        $run->getResult();
    }
}
