<?php

declare(strict_types=1);

namespace Harness\Feature\Update\Activities;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Promise;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const ACTIVITY_COUNT = 5;
const ACTIVITY_RESULT = 6;

#[WorkflowInterface]
class FeatureWorkflow
{
    private int $total = 0;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->total > 0);
        return $this->total;
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate()
    {
        $promises = [];
        for ($i = 0; $i < ACTIVITY_COUNT; ++$i) {
            $promises[] = Workflow::executeActivity(
                'result',
                options: ActivityOptions::new()->withStartToCloseTimeout(10)
            );
        }

        return yield Promise::all($promises)
            ->then(fn(array $results) => $this->total = \array_sum($results));
    }
}

#[ActivityInterface]
class FeatureActivity
{
    #[ActivityMethod('result')]
    public function result(): int
    {
        return ACTIVITY_RESULT;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $updated = $stub->update('my_update')->getValue(0);
        Assert::same($updated, ACTIVITY_COUNT * ACTIVITY_RESULT);
        Assert::same($stub->getResult(), ACTIVITY_COUNT * ACTIVITY_RESULT);
    }
}
