<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\Empty;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use React\Promise\PromiseInterface;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Api\Common\V1\Payload;
use Temporal\Api\Enums\V1\EventType;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Failure\ApplicationFailure;
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
            EmptyActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(10),
        )->nullActivity(null);
    }
}

#[ActivityInterface]
class EmptyActivity
{
    /**
     * @return PromiseInterface<void>
     */
    #[ActivityMethod('null_activity')]
    public function nullActivity(?string $input): void
    {
        // check the null input is serialized correctly
        if ($input !== null) {
            throw new ApplicationFailure('Activity input should be null', 'BadResult', true);
        }
    }
}

class FeatureChecker
{
    #[Check]
    public function check(
        #[Stub('Workflow')]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
    ): void {
        // verify the workflow returns nothing
        $result = $stub->getResult();
        Assert::null($result);

        // get result payload of ActivityTaskScheduled event from workflow history
        $found = false;
        $event = null;
        /** @var HistoryEvent $event */
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            if ($event->getEventType() === EventType::EVENT_TYPE_ACTIVITY_TASK_SCHEDULED) {
                $found = true;
                break;
            }
        }

        Assert::true($found, 'Activity task scheduled event not found');
        $payload = $event->getActivityTaskScheduledEventAttributes()?->getInput()?->getPayloads()[0];
        Assert::isInstanceOf($payload, Payload::class);
        \assert($payload instanceof Payload);

        // load JSON payload from `./payload.json` and compare it to JSON representation of result payload
        $decoded = \json_decode(\file_get_contents(__DIR__ . '/payload.json'), true, 512, JSON_THROW_ON_ERROR);
        Assert::eq(\json_decode($payload->serializeToJsonString(), true, 512, JSON_THROW_ON_ERROR), $decoded);
    }
}
