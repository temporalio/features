<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\Failure;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Api\Common\V1\Payload;
use Temporal\Api\Enums\V1\EventType;
use Temporal\Api\Failure\V1\Failure;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\DataConverter;
use Temporal\Exception\Client\WorkflowFailedException;
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
        )->failActivity(null);
    }
}

#[ActivityInterface]
class EmptyActivity
{
    #[ActivityMethod('fail_activity')]
    public function failActivity(?string $input): never
    {
        throw new ApplicationFailure(
            message: 'main error',
            type: 'MainError',
            nonRetryable: true,
            previous: new ApplicationFailure(
                message: 'cause error',
                type: 'CauseError',
                nonRetryable: true,
            )
        );
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
        try {
            $stub->getResult();
            throw new \Exception('Expected WorkflowFailedException');
        } catch (WorkflowFailedException $e) {
            // do nothing
        }

        // get result payload of ActivityTaskScheduled event from workflow history
        $found = false;
        $event = null;
        /** @var HistoryEvent $event */
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            if ($event->getEventType() === EventType::EVENT_TYPE_ACTIVITY_TASK_FAILED) {
                $found = true;
                break;
            }
        }

        Assert::true($found, 'Activity task failed event not found');
        Assert::true($event->hasActivityTaskFailedEventAttributes());

        $failure = $event->getActivityTaskFailedEventAttributes()?->getFailure();
        Assert::isInstanceOf($failure, Failure::class);
        \assert($failure instanceof Failure);

        $this->checkFailure($failure, 'main error');
        $this->checkFailure($failure->getCause(), 'cause error');
    }

    private function checkFailure(Failure $failure, string $message): void
    {
        Assert::same($failure->getMessage(), 'Encoded failure');
        Assert::isEmpty($failure->getStackTrace());

        $payload = $failure->getEncodedAttributes();
        \assert($payload instanceof Payload);
        Assert::isEmpty($payload->getMetadata()['encoding'], 'json/plain');

        $data = DataConverter::createDefault()->fromPayload($payload, null);
        Assert::same($data['message'], $message);
        Assert::keyExists($data, 'stack_trace');
    }
}
