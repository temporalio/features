<?php

declare(strict_types=1);

namespace Harness\Feature\Update\NonDurableReject;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowUpdateException;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $counter = 0;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->counter === 5);
        return $this->counter;
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate(int $arg): int
    {
        $this->counter += $arg;
        return $this->counter;
    }

    #[Workflow\UpdateValidatorMethod('my_update')]
    public function validateMyUpdate(int $arg): void
    {
        $arg < 0 and throw new \InvalidArgumentException('I *HATE* negative numbers!');
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
    ): void {
        for ($i = 0; $i < 5; $i++) {
            try {
                $stub->update('my_update', -1);
                Assert::fail('Expected exception');
            } catch (WorkflowUpdateException) {
                # Expected
            }

            $stub->update('my_update', 1);
        }

        Assert::same($stub->getResult(), 5);

        # Verify no rejections were written to history since we failed in the validator
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            $event->hasWorkflowExecutionUpdateRejectedEventAttributes() and Assert::fail('Unexpected rejection event');
        }
    }
}
