<?php

declare(strict_types=1);

namespace Harness\Feature\Update\TaskFailure;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowUpdateException;
use Temporal\Exception\Failure\ApplicationFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private bool $done = false;
    private static int $fails = 0;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->done);

        return static::$fails;
    }

    #[Workflow\UpdateMethod('do_update')]
    public function doUpdate(): string
    {
        # Don't use static variables like this. We do here because we need to fail the task a
        # controlled number of times.
        if (static::$fails < 2) {
            ++static::$fails;
            throw new \RuntimeException("I'll fail task");
        }

        throw new ApplicationFailure("I'll fail update", 'task-failure', true);
    }

    #[Workflow\UpdateMethod('throw_or_done')]
    public function throwOrDone(bool $doThrow): void
    {
        $this->done = true;
    }

    #[Workflow\UpdateValidatorMethod('throw_or_done')]
    public function validateThrowOrDone(bool $doThrow): void
    {
        $doThrow and throw new \RuntimeException('This will fail validation, not task');
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        try {
            $stub->update('do_update');
            Assert::fail('Expected validation exception');
        } catch (WorkflowUpdateException $e) {
            Assert::contains($e->getPrevious()?->getMessage(), "I'll fail update");
        }

        try {
            $stub->update('throw_or_done', true);
            Assert::fail('Expected validation exception');
        } catch (WorkflowUpdateException) {
            # Expected
        }

        $stub->update('throw_or_done', false);

        Assert::same($stub->getResult(), 2);
    }
}
