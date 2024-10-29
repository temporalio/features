<?php

declare(strict_types=1);

namespace Harness\Feature\Signal\Basic;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private string $value = '';

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->value !== '');
        return $this->value;
    }

    #[SignalMethod('my_signal')]
    public function mySignal(string $arg)
    {
        $this->value = $arg;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $stub->signal('my_signal', 'arg');
        Assert::same($stub->getResult(), 'arg');
    }
}
