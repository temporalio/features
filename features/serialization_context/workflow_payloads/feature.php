<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\WorkflowPayloads;

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
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const WORKFLOW_INPUT = 'input';
const MEMO_KEY = 'ser-ctx-memo';
const MEMO_VALUE = 'memo';
const QUERY_ARG = 'query-';
const UPDATE_ARG = '-update';
const SIGNAL_DATA = 'signal';

#[WorkflowInterface]
class FeatureWorkflow
{
    private string $input = '';
    private ?string $signaled = null;

    #[WorkflowMethod('SerializationContext_WorkflowPayloads')]
    public function run(SignedValue $input)
    {
        $this->input = (string) $input->value;
        yield Workflow::await(fn(): bool => $this->signaled !== null);

        return new SignedValue("{$this->input}|{$this->signaled}");
    }

    #[Workflow\SignalMethod('append')]
    public function append(SignedValue $data): void
    {
        $this->signaled = (string) $data->value;
    }

    #[Workflow\QueryMethod('prefixed')]
    public function prefixed(SignedValue $prefix): SignedValue
    {
        return new SignedValue($prefix->value . $this->input);
    }

    #[Workflow\UpdateMethod('suffixed')]
    public function suffixed(SignedValue $suffix): SignedValue
    {
        return new SignedValue($this->input . $suffix->value);
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
        #[Stub(
            'SerializationContext_WorkflowPayloads',
            args: [new SignedValue(WORKFLOW_INPUT)],
            memo: [MEMO_KEY => new SignedValue(MEMO_VALUE)],
        )]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [SigningConverter::class],
        )]
        WorkflowStubInterface $stub,
        WorkflowClientInterface $client,
        State $runtime,
    ): void {
        $queried = $stub->query('prefixed', new SignedValue(QUERY_ARG))?->getValue(0);
        Assert::same($queried->value, QUERY_ARG . WORKFLOW_INPUT);

        $updated = $stub->update('suffixed', new SignedValue(UPDATE_ARG))?->getValue(0);
        Assert::same($updated->value, WORKFLOW_INPUT . UPDATE_ARG);

        $stub->signal('append', new SignedValue(SIGNAL_DATA));

        Assert::same($stub->getResult()->value, WORKFLOW_INPUT . '|' . SIGNAL_DATA);

        $events = History::events($client->getWorkflowHistory($stub->getExecution()));
        $expected = Signature::workflow($runtime->namespace, $stub->getExecution()->getID());

        $started = History::find($events, 'WorkflowExecutionStarted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionStartedEventAttributes())->getWorkflowExecutionStartedEventAttributes();
        Assert::same(Signature::first($started->getInput()?->getPayloads()), $expected);
        Assert::same(Signature::ofPayload($started->getMemo()->getFields()[MEMO_KEY] ?? null), $expected);

        $completed = History::find($events, 'WorkflowExecutionCompleted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionCompletedEventAttributes())->getWorkflowExecutionCompletedEventAttributes();
        Assert::same(Signature::first($completed->getResult()?->getPayloads()), $expected);

        $signaled = History::find($events, 'WorkflowExecutionSignaled', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionSignaledEventAttributes())->getWorkflowExecutionSignaledEventAttributes();
        Assert::same(Signature::first($signaled->getInput()?->getPayloads()), $expected);

        $accepted = History::find($events, 'WorkflowExecutionUpdateAccepted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionUpdateAcceptedEventAttributes())->getWorkflowExecutionUpdateAcceptedEventAttributes();
        Assert::same(Signature::first($accepted->getAcceptedRequest()->getInput()->getArgs()->getPayloads()), $expected);

        $updateCompleted = History::find($events, 'WorkflowExecutionUpdateCompleted', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionUpdateCompletedEventAttributes())->getWorkflowExecutionUpdateCompletedEventAttributes();
        Assert::same(Signature::first($updateCompleted->getOutcome()->getSuccess()->getPayloads()), $expected);
    }
}
