<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\AsyncActivityCompletion;

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
use Temporal\Client\ClientOptions;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\ActivitySerializationContext;
use Temporal\DataConverter\BinaryConverter;
use Temporal\DataConverter\DataConverter;
use Temporal\DataConverter\JsonConverter;
use Temporal\DataConverter\NullConverter;
use Temporal\DataConverter\ProtoConverter;
use Temporal\DataConverter\ProtoJsonConverter;
use Temporal\DataConverter\Type;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const ACTIVITY_RESULT = 'completed-out-of-band';
const HEARTBEAT_DATA = 'beat';

#[ActivityInterface(prefix: 'SerializationContext_AsyncActivityCompletion.')]
class FeatureActivity
{
    #[ActivityMethod('run')]
    public function run(): string
    {
        Activity::doNotCompleteOnReturn();

        return '';
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_AsyncActivityCompletion')]
    public function run()
    {
        return yield Workflow::executeActivity(
            'SerializationContext_AsyncActivityCompletion.run',
            [],
            ActivityOptions::new()
                ->withStartToCloseTimeout(60)
                ->withHeartbeatTimeout(30),
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
        #[Stub('SerializationContext_AsyncActivityCompletion')]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        $execution = $stub->getExecution();

        [$started, $scheduled] = self::awaitScheduled($client, $stub);

        $context = new ActivitySerializationContext(
            namespace: $runtime->namespace,
            activityType: $scheduled->getActivityType()->getName(),
            taskQueue: $scheduled->getTaskQueue()->getName(),
            workflowId: $execution->getID(),
            workflowType: $started->getWorkflowType()->getName(),
            isLocal: false,
        );

        $signingClient = WorkflowClient::create(
            serviceClient: $client->getServiceClient(),
            options: (new ClientOptions())->withNamespace($runtime->namespace),
            converter: new DataConverter(
                new SigningConverter(),
                new NullConverter(),
                new BinaryConverter(),
                new ProtoConverter(),
                new ProtoJsonConverter(),
                new JsonConverter(),
            ),
        );

        $completion = $signingClient->newActivityCompletionClient()->withContext($context);

        $completion->recordHeartbeat(
            $execution->getID(),
            $execution->getRunID(),
            $scheduled->getActivityId(),
            new SignedValue(HEARTBEAT_DATA),
        );

        self::assertHeartbeatDetails($stub);

        $completion->complete(
            $execution->getID(),
            $execution->getRunID(),
            $scheduled->getActivityId(),
            new SignedValue(ACTIVITY_RESULT),
        );

        Assert::same($stub->getResult()->value, ACTIVITY_RESULT);

        $expected = Signature::activity(
            $runtime->namespace,
            $execution->getID(),
            $started->getWorkflowType()->getName(),
            $scheduled->getActivityType()->getName(),
            $scheduled->getTaskQueue()->getName(),
            false,
        );

        $events = History::events($client->getWorkflowHistory($execution));
        $completed = History::find($events, 'ActivityTaskCompleted', fn(HistoryEvent $e): bool =>
            $e->hasActivityTaskCompletedEventAttributes())->getActivityTaskCompletedEventAttributes();
        Assert::same(Signature::first($completed->getResult()?->getPayloads()), $expected);
    }

    private static function assertHeartbeatDetails(WorkflowStubInterface $stub): void
    {
        $deadline = \microtime(true) + 30;
        do {
            foreach ($stub->describe()->pendingActivities as $pending) {
                if (!$pending->heartbeatDetails->isEmpty()) {
                    Assert::same($pending->heartbeatDetails->getValue(0)->value, HEARTBEAT_DATA);

                    return;
                }
            }

            \usleep(100_000);
        } while (\microtime(true) < $deadline);

        throw new \RuntimeException('heartbeat details were never recorded');
    }

    /**
     * @return array{0: \Temporal\Api\History\V1\WorkflowExecutionStartedEventAttributes, 1: \Temporal\Api\History\V1\ActivityTaskScheduledEventAttributes}
     */
    private static function awaitScheduled(WorkflowClientInterface $client, WorkflowStubInterface $stub): array
    {
        $deadline = \microtime(true) + 30;
        do {
            $started = null;
            $scheduled = null;
            foreach (History::events($client->getWorkflowHistory($stub->getExecution())) as $event) {
                $event->hasWorkflowExecutionStartedEventAttributes()
                    and $started = $event->getWorkflowExecutionStartedEventAttributes();
                $event->hasActivityTaskScheduledEventAttributes()
                    and $scheduled = $event->getActivityTaskScheduledEventAttributes();
            }

            if ($started !== null && $scheduled !== null) {
                return [$started, $scheduled];
            }

            \usleep(100_000);
        } while (\microtime(true) < $deadline);

        throw new \RuntimeException('activity was never scheduled');
    }
}
