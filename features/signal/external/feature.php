<?php

declare(strict_types=1);

namespace Harness\Feature\Signal\External;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const SIGNAL_DATA = 'Signaled!';

#[WorkflowInterface]
class FeatureWorkflow
{
    private ?string $result = null;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->result !== null);
        return $this->result;
    }

    #[SignalMethod('my_signal')]
    public function mySignal(string $arg)
    {
        $this->result = $arg;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $stub->signal('my_signal', SIGNAL_DATA);
        Assert::same($stub->getResult(), SIGNAL_DATA);
    }
}
