<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext\ExternalSignal;

use Harness\Attribute\Check;
use Harness\Feature\SerializationContext\History;
use Harness\Feature\SerializationContext\SignedValue;
use Harness\Feature\SerializationContext\Signature;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\ClientOptions;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\DataConverter\BinaryConverter;
use Temporal\DataConverter\DataConverter;
use Temporal\DataConverter\JsonConverter;
use Temporal\DataConverter\NullConverter;
use Temporal\DataConverter\ProtoConverter;
use Temporal\DataConverter\ProtoJsonConverter;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowExecution;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

require_once __DIR__ . '/../sercontext/sercontext.php';

final class SigningConverter extends \Harness\Feature\SerializationContext\SigningConverter {}

const SIGNAL_DATA = 'signaled';
const RECEIVER_SUFFIX = '_receiver';

#[WorkflowInterface]
class Receiver
{
    private ?string $received = null;

    #[WorkflowMethod('SerializationContext_ExternalSignal_Receiver')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->received !== null);

        return new SignedValue($this->received);
    }

    #[Workflow\SignalMethod('external')]
    public function external(SignedValue $data): void
    {
        $this->received = (string) $data->value;
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('SerializationContext_ExternalSignal')]
    public function run()
    {
        $targetId = Workflow::getInfo()->taskQueue . RECEIVER_SUFFIX;

        yield Workflow::newUntypedExternalWorkflowStub(new WorkflowExecution($targetId))
            ->signal('external', [new SignedValue(SIGNAL_DATA)]);

        return $targetId;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        WorkflowClientInterface $client,
        Feature $feature,
        State $runtime,
    ): void {
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

        $receiverId = $feature->taskQueue . RECEIVER_SUFFIX;
        $receiver = $signingClient->newUntypedWorkflowStub(
            'SerializationContext_ExternalSignal_Receiver',
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue)->withWorkflowId($receiverId),
        );
        $signingClient->start($receiver);

        $sender = $signingClient->newUntypedWorkflowStub(
            'SerializationContext_ExternalSignal',
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue),
        );
        $signingClient->start($sender);

        Assert::same($sender->getResult(), $receiverId);
        Assert::same($receiver->getResult()->value, SIGNAL_DATA);

        $expected = Signature::workflow($runtime->namespace, $receiverId);
        Assert::notSame($expected, Signature::workflow($runtime->namespace, $sender->getExecution()->getID()));

        $senderEvents = History::events($client->getWorkflowHistory($sender->getExecution()));
        $initiated = History::find($senderEvents, 'SignalExternalWorkflowExecutionInitiated', fn(HistoryEvent $e): bool =>
            $e->hasSignalExternalWorkflowExecutionInitiatedEventAttributes())
            ->getSignalExternalWorkflowExecutionInitiatedEventAttributes();
        Assert::same(Signature::first($initiated->getInput()?->getPayloads()), $expected);

        $receiverEvents = History::events($client->getWorkflowHistory($receiver->getExecution()));
        $signaled = History::find($receiverEvents, 'WorkflowExecutionSignaled', fn(HistoryEvent $e): bool =>
            $e->hasWorkflowExecutionSignaledEventAttributes())->getWorkflowExecutionSignaledEventAttributes();
        Assert::same(Signature::first($signaled->getInput()?->getPayloads()), $expected);
    }
}
