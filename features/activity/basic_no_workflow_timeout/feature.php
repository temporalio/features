<?php

declare(strict_types=1);

namespace Harness\Feature\Activity\BasicNoWorkflowTimeout;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::newActivityStub(
            FeatureActivity::class,
            ActivityOptions::new()->withScheduleToCloseTimeout('1 minute'),
        )->echo();

        return yield Workflow::newActivityStub(
            FeatureActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout('1 minute'),
        )->echo();
    }
}

#[ActivityInterface]
class FeatureActivity
{
    #[ActivityMethod('echo')]
    public function echo(): string
    {
        return 'echo';
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(#[Stub('Workflow')] WorkflowStubInterface $stub): void
    {
        Assert::same($stub->getResult(), 'echo');
    }
}
