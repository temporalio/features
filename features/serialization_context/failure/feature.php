<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\Failure;

use Harness\Attribute\Check;
use Harness\Runtime\State;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Harness\Feature\SerializationContext\History;
use Harness\Feature\SerializationContext\Signature;
use Harness\Feature\SerializationContext\SignedValue;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Common\RetryOptions;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\EncodedValues;
use Temporal\Exception\Client\WorkflowFailedException;
use Temporal\Exception\Failure\ActivityFailure;
use Temporal\Exception\Failure\ApplicationFailure;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const ACTIVITY_DETAIL = 'activity-detail';
const WORKFLOW_DETAIL = 'workflow-detail';

#[ActivityInterface(prefix: 'SerializationContext_Failure.')]
class FeatureActivity
{
    #[ActivityMethod('fail')]
    public function fail(): void
    {
        throw new ApplicationFailure(
            'activity failed',
            'ActivityError',
            true,
            EncodedValues::fromValues([new SignedValue(ACTIVITY_DETAIL)]),
        );
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_Failure')]
    public function run()
    {
        try {
            yield Workflow::executeActivity(
                'SerializationContext_Failure.fail',
                [],
                ActivityOptions::new()
                    ->withStartToCloseTimeout(10)
                    ->withRetryOptions(RetryOptions::new()->withMaximumAttempts(1)),
            );
        } catch (ActivityFailure) {
            throw new ApplicationFailure(
                'workflow failed',
                'WorkflowError',
                true,
                EncodedValues::fromValues([new SignedValue(WORKFLOW_DETAIL)]),
            );
        }
    }
}

class FeatureChecker
{
    public function pipelineProvider(): PipelineProvider
    {
        return new SimplePipelineProvider();
    }

    #[Check]
    public static function check(
        #[Stub('SerializationContext_Failure')]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        try {
            $stub->getResult();
            throw new \RuntimeException('expected the workflow to fail');
        } catch (WorkflowFailedException) {
        }

        $events = History::events($client->getWorkflowHistory($stub->getExecution()));
        $workflowId = $stub->getExecution()->getID();

        $started = History::find($events, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        $scheduled = History::find($events, 'ActivityTaskScheduled', fn(HistoryEvent $e): bool =>
            $e->hasActivityTaskScheduledEventAttributes())->getActivityTaskScheduledEventAttributes();

        $activityExpected = Signature::activity(
            $runtime->namespace,
            $workflowId,
            $started->getWorkflowType()->getName(),
            $scheduled->getActivityType()->getName(),
            $scheduled->getTaskQueue()->getName(),
            false,
        );

        $activityFailed = History::find($events, 'ActivityTaskFailed', fn(HistoryEvent $e): bool =>
            $e->hasActivityTaskFailedEventAttributes())->getActivityTaskFailedEventAttributes();
        Assert::same(
            Signature::first($activityFailed->getFailure()?->getApplicationFailureInfo()?->getDetails()?->getPayloads()),
            $activityExpected,
        );

        $workflowFailed = History::find($events, 'WorkflowExecutionFailed', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionFailedEventAttributes())->getWorkflowExecutionFailedEventAttributes();
        Assert::same(
            Signature::first($workflowFailed->getFailure()?->getApplicationFailureInfo()?->getDetails()?->getPayloads()),
            Signature::workflow($runtime->namespace, $workflowId),
        );
    }
}
