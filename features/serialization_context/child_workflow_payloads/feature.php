<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\ChildWorkflowPayloads;

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
use Temporal\Workflow\ChildWorkflowOptions;
use Temporal\Workflow\WorkflowExecution;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const WORKFLOW_INPUT = 'hello';
const CHILD_ID_SUFFIX = '_child';
const CHILD_RESULT_TAG = '|child';

#[WorkflowInterface]
class ChildWorkflow
{
    #[WorkflowMethod('SerializationContext_ChildWorkflowPayloads_Child')]
    public function run(SignedValue $input)
    {
        return new SignedValue($input->value . CHILD_RESULT_TAG);
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_ChildWorkflowPayloads')]
    public function run(SignedValue $input)
    {
        $child = Workflow::newUntypedChildWorkflowStub(
            'SerializationContext_ChildWorkflowPayloads_Child',
            ChildWorkflowOptions::new()
                ->withWorkflowId(Workflow::getInfo()->execution->getID() . CHILD_ID_SUFFIX)
                ->withWorkflowRunTimeout('1 minute'),
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
        #[Stub('SerializationContext_ChildWorkflowPayloads', args: [new SignedValue(WORKFLOW_INPUT)])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        Assert::same($stub->getResult()->value, WORKFLOW_INPUT . CHILD_RESULT_TAG);

        $childId = $stub->getExecution()->getID() . CHILD_ID_SUFFIX;
        $expected = Signature::workflow($runtime->namespace, $childId);
        Assert::notSame($expected, Signature::workflow($runtime->namespace, $stub->getExecution()->getID()));

        $parentEvents = History::events($client->getWorkflowHistory($stub->getExecution()));
        $initiated = History::find($parentEvents, 'StartChildWorkflowExecutionInitiated', fn(HistoryEvent $e): bool =>
            $e->hasStartChildWorkflowExecutionInitiatedEventAttributes())
            ->getStartChildWorkflowExecutionInitiatedEventAttributes();
        Assert::same(Signature::first($initiated->getInput()?->getPayloads()), $expected);

        $childCompleted = History::find($parentEvents, 'ChildWorkflowExecutionCompleted', fn(HistoryEvent $e): bool =>
            $e->hasChildWorkflowExecutionCompletedEventAttributes())
            ->getChildWorkflowExecutionCompletedEventAttributes();
        Assert::same(Signature::first($childCompleted->getResult()?->getPayloads()), $expected);

        $childStartedInParent = History::find($parentEvents, 'ChildWorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasChildWorkflowExecutionStartedEventAttributes())
            ->getChildWorkflowExecutionStartedEventAttributes()->getWorkflowExecution();
        $childExecution = new WorkflowExecution(
            $childStartedInParent->getWorkflowId(),
            $childStartedInParent->getRunId(),
        );
        $childEvents = History::events($client->getWorkflowHistory($childExecution));
        $childStarted = History::find($childEvents, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        Assert::same(Signature::first($childStarted->getInput()?->getPayloads()), $expected);
    }
}
