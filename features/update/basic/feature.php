<?php

declare(strict_types=1);

namespace Harness\Feature\Update\Basic;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private bool $done = false;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->done);
        return 'Hello, world!';
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate()
    {
        $this->done = true;
        return 'Updated';
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $updated = $stub->update('my_update')->getValue(0);
        Assert::same($updated, 'Updated');
        Assert::same($stub->getResult(), 'Hello, world!');
    }
}
