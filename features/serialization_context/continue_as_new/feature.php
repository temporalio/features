<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\ContinueAsNew;

use Harness\Attribute\Check;
use Harness\Runtime\State;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Harness\Feature\SerializationContext\History;
use Harness\Feature\SerializationContext\Signature;
use Harness\Feature\SerializationContext\SignedValue;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowExecution;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const FINAL_RESULT = 'done';

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_ContinueAsNew')]
    public function run(SignedValue $remaining)
    {
        if ($remaining->value > 0) {
            return yield Workflow::continueAsNew(
                'SerializationContext_ContinueAsNew',
                [new SignedValue($remaining->value - 1)],
            );
        }

        return new SignedValue(FINAL_RESULT);
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
        #[Stub('SerializationContext_ContinueAsNew', args: [new SignedValue(1)])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        $firstExecution = $stub->getExecution();
        Assert::same($stub->getResult()->value, FINAL_RESULT);
        $lastExecution = $stub->getExecution();

        $workflowId = $firstExecution->getID();
        $expected = Signature::workflow($runtime->namespace, $workflowId);

        $firstRunEvents = History::events($client->getWorkflowHistory($firstExecution));
        $continued = History::find($firstRunEvents, 'WorkflowExecutionContinuedAsNew', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionContinuedAsNewEventAttributes())
            ->getWorkflowExecutionContinuedAsNewEventAttributes();
        Assert::same(Signature::first($continued->getInput()?->getPayloads()), $expected);

        $lastRunEvents = History::events($client->getWorkflowHistory($lastExecution));
        $started = History::find($lastRunEvents, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        Assert::same(Signature::first($started->getInput()?->getPayloads()), $expected);

        $completed = History::find($lastRunEvents, 'WorkflowExecutionCompleted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionCompletedEventAttributes())->getWorkflowExecutionCompletedEventAttributes();
        Assert::same(Signature::first($completed->getResult()?->getPayloads()), $expected);
    }
}
