<?php

declare(strict_types=1);

namespace Harness\Feature\Nexus\SyncOperationError;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Exception\Failure\ApplicationFailure;
use Temporal\Exception\Failure\NexusOperationFailure;
use Temporal\Nexus\Attribute\Operation;
use Temporal\Nexus\Attribute\Service;
use Temporal\Nexus\Exception\OperationException;
use Temporal\Workflow;
use Temporal\Workflow\NexusOperationOptions;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const ERROR_TYPE = 'TestFailure';
const ERROR_MESSAGE = 'deliberate failure';

#[Service(name: 'test-service')]
class TestService
{
    #[Operation(name: 'fail')]
    public function fail(string $name): string
    {
        throw OperationException::failed(
            ERROR_MESSAGE,
            new ApplicationFailure(ERROR_MESSAGE, ERROR_TYPE, true),
        );
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(string $endpoint)
    {
        $stub = Workflow::newUntypedNexusOperationStub(
            NexusOperationOptions::new()
                ->withEndpoint($endpoint)
                ->withService('test-service')
                ->withScheduleToCloseTimeout('1 minute'),
        );

        try {
            yield $stub->execute('fail', ['world'], 'string');
        } catch (NexusOperationFailure $e) {
            $cause = self::findApplicationFailure($e, ERROR_TYPE);
            if ($cause === null) {
                throw new \RuntimeException('expected an application error cause of type ' . ERROR_TYPE);
            }

            if (!\str_contains($cause->getOriginalMessage(), ERROR_MESSAGE)) {
                throw new \RuntimeException(
                    'expected the original failure message, got: ' . $cause->getOriginalMessage(),
                );
            }

            return $cause->getType() . ': ' . ERROR_MESSAGE;
        }

        throw new \RuntimeException('expected the operation to fail');
    }

    private static function findApplicationFailure(\Throwable $error, string $type): ?ApplicationFailure
    {
        for ($current = $error->getPrevious(); $current !== null; $current = $current->getPrevious()) {
            if ($current instanceof ApplicationFailure && $current->getType() === $type) {
                return $current;
            }
        }

        return null;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(WorkflowClientInterface $client, Feature $feature): void
    {
        Assert::notNull($feature->nexusEndpoint, 'Nexus endpoint is not provided by the runner');

        $stub = $client->newUntypedWorkflowStub(
            'Workflow',
            WorkflowOptions::new()
                ->withTaskQueue($feature->taskQueue)
                ->withWorkflowExecutionTimeout('1 minute'),
        );
        $client->start($stub, $feature->nexusEndpoint);

        Assert::same($stub->getResult('string'), ERROR_TYPE . ': ' . ERROR_MESSAGE);

        $events = \iterator_to_array($client->getWorkflowHistory($stub->getExecution())->getEvents(), false);

        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationFailedEventAttributes()),
            'NexusOperationFailed event is missing',
        );
        Assert::false(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationCompletedEventAttributes()),
            'Unexpected NexusOperationCompleted event for a failed operation',
        );
    }

    /**
     * @param list<HistoryEvent> $events
     * @param callable(HistoryEvent): bool $predicate
     */
    private static function hasEvent(array $events, callable $predicate): bool
    {
        foreach ($events as $event) {
            if ($predicate($event)) {
                return true;
            }
        }

        return false;
    }
}
