<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\ActivityPayloads;

use Harness\Attribute\Check;
use Harness\Runtime\State;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Harness\Feature\SerializationContext\History;
use Harness\Feature\SerializationContext\Signature;
use Harness\Feature\SerializationContext\SignedValue;
use Temporal\Activity;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Common\RetryOptions;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\Type;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const WORKFLOW_INPUT = 'hello';
const HEARTBEAT_DATA = 'beat';

#[ActivityInterface(prefix: 'SerializationContext_ActivityPayloads.')]
class FeatureActivity
{
    #[ActivityMethod('run')]
    public function run(SignedValue $input): SignedValue
    {
        if (Activity::hasHeartbeatDetails()) {
            $beat = Activity::getHeartbeatDetails(SignedValue::class);

            return new SignedValue($input->value . '|' . $beat->value);
        }

        Activity::heartbeat(new SignedValue(HEARTBEAT_DATA));
        throw new \RuntimeException('retrying to read back heartbeat details');
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_ActivityPayloads')]
    public function run(SignedValue $input)
    {
        return yield Workflow::executeActivity(
            'SerializationContext_ActivityPayloads.run',
            [$input],
            ActivityOptions::new()
                ->withStartToCloseTimeout(10)
                ->withHeartbeatTimeout(5)
                ->withRetryOptions(RetryOptions::new()->withInitialInterval(1)->withMaximumAttempts(2)),
            SignedValue::class,
        );
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
        #[Stub('SerializationContext_ActivityPayloads', args: [new SignedValue(WORKFLOW_INPUT)])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        Assert::same($stub->getResult()->value, WORKFLOW_INPUT . '|' . HEARTBEAT_DATA);

        $events = History::events($client->getWorkflowHistory($stub->getExecution()));
        $workflowId = $stub->getExecution()->getID();

        $started = History::find($events, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        $scheduled = History::find($events, 'ActivityTaskScheduled', fn(HistoryEvent $e): bool =>
            $e->hasActivityTaskScheduledEventAttributes())->getActivityTaskScheduledEventAttributes();

        $expected = Signature::activity(
            $runtime->namespace,
            $workflowId,
            $started->getWorkflowType()->getName(),
            $scheduled->getActivityType()->getName(),
            $scheduled->getTaskQueue()->getName(),
            false,
        );
        Assert::same(Signature::first($scheduled->getInput()?->getPayloads()), $expected);

        $completed = History::find($events, 'ActivityTaskCompleted', fn(HistoryEvent $e): bool =>
            $e->hasActivityTaskCompletedEventAttributes())->getActivityTaskCompletedEventAttributes();
        Assert::same(Signature::first($completed->getResult()?->getPayloads()), $expected);

        $workflowCompleted = History::find($events, 'WorkflowExecutionCompleted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionCompletedEventAttributes())->getWorkflowExecutionCompletedEventAttributes();
        Assert::same(
            Signature::first($workflowCompleted->getResult()?->getPayloads()),
            Signature::workflow($runtime->namespace, $workflowId),
        );
    }
}
