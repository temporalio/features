<?php

declare(strict_types=1);

namespace Harness\Feature\Query\SuccessfulQuery;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\QueryMethod;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

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
    public static function check(#[Stub('Workflow')] WorkflowStubInterface $stub): void
    {
        Assert::same($stub->query('get_counter')?->getValue(0), 0);

        $stub->signal('inc_counter');
        Assert::same($stub->query('get_counter')?->getValue(0), 1);

        $stub->signal('inc_counter');
        $stub->signal('inc_counter');
        $stub->signal('inc_counter');
        Assert::same($stub->query('get_counter')?->getValue(0), 4);

        $stub->signal('finish');
        $stub->getResult();
    }
}
