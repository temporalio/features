<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\ChildWorkflowPayloadsDefaultId;

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
use Temporal\DataConverter\Type;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowExecution;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const WORKFLOW_INPUT = 'hello';
const CHILD_RESULT_TAG = '|child';

#[WorkflowInterface]
class ChildWorkflow
{
    #[WorkflowMethod('SerializationContext_ChildWorkflowPayloadsDefaultId_Child')]
    public function run(SignedValue $input)
    {
        return new SignedValue($input->value . CHILD_RESULT_TAG);
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_ChildWorkflowPayloadsDefaultId')]
    public function run(SignedValue $input)
    {
        $child = Workflow::newUntypedChildWorkflowStub(
            'SerializationContext_ChildWorkflowPayloadsDefaultId_Child',
            Workflow\ChildWorkflowOptions::new()->withWorkflowRunTimeout('1 minute'),
        );

        return yield $child->execute([$input], SignedValue::class);
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
        #[Stub('SerializationContext_ChildWorkflowPayloadsDefaultId', args: [new SignedValue(WORKFLOW_INPUT)])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        Assert::same($stub->getResult()->value, WORKFLOW_INPUT . CHILD_RESULT_TAG);

        $parentEvents = History::events($client->getWorkflowHistory($stub->getExecution()));

        $childStartedInParent = History::find($parentEvents, 'ChildWorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasChildWorkflowExecutionStartedEventAttributes())
            ->getChildWorkflowExecutionStartedEventAttributes()->getWorkflowExecution();
        $childId = $childStartedInParent->getWorkflowId();

        Assert::notSame($childId, $stub->getExecution()->getID());
        Assert::notEmpty($childId);
        $expected = Signature::workflow($runtime->namespace, $childId);
        Assert::notSame($expected, Signature::workflow($runtime->namespace, $stub->getExecution()->getID()));

        $initiated = History::find($parentEvents, 'StartChildWorkflowExecutionInitiated', fn(HistoryEvent $e): bool =>
            $e->hasStartChildWorkflowExecutionInitiatedEventAttributes())
            ->getStartChildWorkflowExecutionInitiatedEventAttributes();
        Assert::same(Signature::first($initiated->getInput()?->getPayloads()), $expected);

        $childCompleted = History::find($parentEvents, 'ChildWorkflowExecutionCompleted', fn(HistoryEvent $e): bool =>
            $e->hasChildWorkflowExecutionCompletedEventAttributes())
            ->getChildWorkflowExecutionCompletedEventAttributes();
        Assert::same(Signature::first($childCompleted->getResult()?->getPayloads()), $expected);

        $childExecution = new WorkflowExecution($childId, $childStartedInParent->getRunId());
        $childEvents = History::events($client->getWorkflowHistory($childExecution));
        $childStarted = History::find($childEvents, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        Assert::same(Signature::first($childStarted->getInput()?->getPayloads()), $expected);
    }
}
