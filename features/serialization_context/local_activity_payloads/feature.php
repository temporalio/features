<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\LocalActivityPayloads;

use Harness\Attribute\Check;
use Harness\Runtime\State;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Harness\Feature\SerializationContext\History;
use Harness\Feature\SerializationContext\Signature;
use Harness\Feature\SerializationContext\SignedValue;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\LocalActivityOptions;
use Temporal\Api\History\V1\HistoryEvent;
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
const ACTIVITY_NAME = 'SerializationContext_LocalActivityPayloads.run';

#[ActivityInterface(prefix: 'SerializationContext_LocalActivityPayloads.')]
class FeatureActivity
{
    #[ActivityMethod('run')]
    public function run(SignedValue $input): SignedValue
    {
        return new SignedValue($input->value . '|local');
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_LocalActivityPayloads')]
    public function run(SignedValue $input)
    {
        return yield Workflow::executeActivity(
            ACTIVITY_NAME,
            [$input],
            LocalActivityOptions::new()->withStartToCloseTimeout(10),
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
        #[Stub('SerializationContext_LocalActivityPayloads', args: [new SignedValue(WORKFLOW_INPUT)])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        Assert::same($stub->getResult()->value, WORKFLOW_INPUT . '|local');

        $events = History::events($client->getWorkflowHistory($stub->getExecution()));
        $workflowId = $stub->getExecution()->getID();

        $started = History::find($events, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();

        $marker = History::find($events, 'LocalActivity marker', fn(HistoryEvent $e): bool =>
            $e->hasMarkerRecordedEventAttributes()
            && $e->getMarkerRecordedEventAttributes()->getMarkerName() === 'LocalActivity')
            ->getMarkerRecordedEventAttributes();
        $details = $marker->getDetails();

        Assert::same(
            Signature::first(($details['result'] ?? null)?->getPayloads()),
            Signature::activity(
                $runtime->namespace,
                $workflowId,
                $started->getWorkflowType()->getName(),
                ACTIVITY_NAME,
                $started->getTaskQueue()->getName(),
                true,
            ),
        );
    }
}
