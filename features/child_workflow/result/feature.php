<?php

declare(strict_types=1);

namespace Harness\Feature\ChildWorkflow\Result;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class MainWorkflow
{
    #[WorkflowMethod('MainWorkflow')]
    public function run()
    {
        return yield Workflow::newChildWorkflowStub(ChildWorkflow::class)->run('Test');
    }
}

#[WorkflowInterface]
class ChildWorkflow
{
    #[WorkflowMethod('ChildWorkflow')]
    public function run(string $input)
    {
        return $input;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(#[Stub('MainWorkflow')] WorkflowStubInterface $stub): void
    {
        Assert::same($stub->getResult(), 'Test');
    }
}
