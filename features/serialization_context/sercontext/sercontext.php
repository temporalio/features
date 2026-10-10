<?php

declare(strict_types=1);

namespace Harness\Feature\SerializationContext;

use Temporal\Api\Common\V1\Payload;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\Workflow\WorkflowExecutionHistory;
use Temporal\DataConverter\ActivitySerializationContext;
use Temporal\DataConverter\HasWorkflowSerializationContext;
use Temporal\DataConverter\JsonConverter;
use Temporal\DataConverter\PayloadConverterInterface;
use Temporal\DataConverter\SerializationContext;
use Temporal\DataConverter\SerializationContextAwareInterface;
use Temporal\DataConverter\Type;

final class SignedValue
{
    public function __construct(
        public mixed $value,
    ) {}
}

final class Signature
{
    public static function workflow(string $namespace, ?string $workflowId): string
    {
        return \sprintf('wf|%s|%s', $namespace, $workflowId ?? '');
    }

    public static function activity(
        string $namespace,
        ?string $workflowId,
        ?string $workflowType,
        string $activityType,
        string $taskQueue,
        bool $isLocal,
    ): string {
        return \sprintf(
            'act|%s|%s|%s|%s|%s|%s',
            $namespace,
            $workflowId ?? '',
            $workflowType ?? '',
            $activityType,
            $taskQueue,
            $isLocal ? 'true' : 'false',
        );
    }

    public static function of(?SerializationContext $context): string
    {
        if ($context instanceof ActivitySerializationContext) {
            return self::activity(
                $context->namespace,
                $context->workflowId,
                $context->workflowType,
                $context->activityType,
                $context->taskQueue,
                $context->isLocal,
            );
        }

        if ($context instanceof HasWorkflowSerializationContext) {
            return self::workflow($context->getNamespace(), $context->getWorkflowId());
        }

        return SigningConverter::NO_CONTEXT;
    }

    public static function ofPayload(?Payload $payload): string
    {
        return $payload === null ? '' : ($payload->getMetadata()[SigningConverter::METADATA_KEY] ?? '');
    }

    /**
     * @param iterable<Payload>|null $payloads
     */
    public static function first(?iterable $payloads): string
    {
        foreach ($payloads ?? [] as $payload) {
            return self::ofPayload($payload);
        }

        return '';
    }
}

class SigningConverter implements PayloadConverterInterface, SerializationContextAwareInterface
{
    public const ENCODING = 'ser-context/signed';
    public const METADATA_KEY = 'ctx-signature';
    public const NO_CONTEXT = 'none';

    private JsonConverter $json;
    private string $signature = self::NO_CONTEXT;

    public function __construct()
    {
        $this->json = new JsonConverter();
    }

    public function getEncodingType(): string
    {
        return self::ENCODING;
    }

    public function getSerializationContext(): ?SerializationContext
    {
        return null;
    }

    public function withSerializationContext(?SerializationContext $context): static
    {
        $clone = clone $this;
        $clone->signature = Signature::of($context);
        return $clone;
    }

    public function toPayload($value): ?Payload
    {
        if (!$value instanceof SignedValue) {
            return null;
        }

        return (new Payload())
            ->setData((string) \json_encode($value->value))
            ->setMetadata([
                'encoding' => self::ENCODING,
                self::METADATA_KEY => $this->signature,
            ]);
    }

    public function fromPayload(Payload $payload, Type $type): mixed
    {
        $encoded = Signature::ofPayload($payload);
        if ($encoded !== $this->signature) {
            throw new \RuntimeException(\sprintf(
                'serialization context mismatch: payload encoded as "%s", decoded as "%s"',
                $encoded,
                $this->signature,
            ));
        }

        return new SignedValue(\json_decode($payload->getData(), false, 512, \JSON_THROW_ON_ERROR));
    }
}

final class History
{
    /**
     * @return list<HistoryEvent>
     */
    public static function events(WorkflowExecutionHistory $history): array
    {
        return \iterator_to_array($history->getEvents(), false);
    }

    /**
     * @param list<HistoryEvent> $events
     * @param callable(HistoryEvent): bool $predicate
     */
    public static function find(array $events, string $name, callable $predicate): HistoryEvent
    {
        foreach ($events as $event) {
            if ($predicate($event)) {
                return $event;
            }
        }

        throw new \RuntimeException("no {$name} event in history");
    }
}
