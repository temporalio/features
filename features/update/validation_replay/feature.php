<?php

declare(strict_types=1);

namespace Harness\Feature\Update\ValidationReplay;

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

    # Don't use static variables like this.
    private static int $validations = 0;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->done);

        return static::$validations;
    }

    #[Workflow\UpdateMethod('do_update')]
    public function doUpdate(): void
    {
        if (static::$validations === 0) {
            ++static::$validations;
            throw new \RuntimeException("I'll fail task");
        }

        $this->done = true;
    }

    #[Workflow\UpdateValidatorMethod('do_update')]
    public function validateDoUpdate(): void
    {
        if (static::$validations > 1) {
            throw new \RuntimeException('I would reject if I even ran :|');
        }
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $stub->update('do_update');
        Assert::same($stub->getResult(), 1);
    }
}
