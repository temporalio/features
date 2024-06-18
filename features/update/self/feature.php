<?php

declare(strict_types=1);

namespace Harness\Feature\Update\Self;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowClientInterface;
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
        yield Workflow::executeActivity(
            'result',
            options: ActivityOptions::new()->withStartToCloseTimeout(2)
        );

        yield Workflow::await(fn(): bool => $this->done);

        return 'Hello, world!';
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate()
    {
        $this->done = true;
    }
}

#[ActivityInterface]
class FeatureActivity
{
    public function __construct(
        private WorkflowClientInterface $client,
    ) {}

    #[ActivityMethod('result')]
    public function result(): void
    {
        $this->client->newUntypedRunningWorkflowStub(
            workflowID: Activity::getInfo()->workflowExecution->getID(),
            workflowType: Activity::getInfo()->workflowType->name,
        )->update('my_update');
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        Assert::same($stub->getResult(), 'Hello, world!');
    }
}
