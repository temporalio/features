<?php

declare(strict_types=1);

namespace Harness\Feature\Update\TaskFailure;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Harness\Exception\SkipTest;
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
            throw new class extends \Error {
                public function __construct()
                {
                    parent::__construct("I'll fail task");
                }
            };
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
    public static function retryableException(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        throw new SkipTest('TODO: doesn\'t pass in some cases');

        try {
            $stub->update('do_update');
            throw new \RuntimeException('Expected validation exception');
        } catch (WorkflowUpdateException $e) {
            Assert::contains($e->getPrevious()?->getMessage(), "I'll fail update");
        } finally {
            # Finish Workflow
            $stub->update('throw_or_done', doThrow: false);
        }

        Assert::same($stub->getResult(), 2);
    }

    #[Check]
    public static function validationException(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        try {
            $stub->update('throw_or_done', true);
            throw new \RuntimeException('Expected validation exception');
        } catch (WorkflowUpdateException) {
            # Expected
        } finally {
            # Finish Workflow
            $stub->update('throw_or_done', doThrow: false);
        }
    }
}
