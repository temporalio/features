<?php

declare(strict_types=1);

namespace Harness\Feature\Nexus\SyncSuccess;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Nexus\Attribute\Operation;
use Temporal\Nexus\Attribute\Service;
use Temporal\Workflow;
use Temporal\Workflow\NexusOperationOptions;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[Service(name: 'test-service')]
interface TestService
{
    #[Operation(name: 'say-hello')]
    public function sayHello(string $name): string;
}

final class TestServiceImpl implements TestService
{
    public function sayHello(string $name): string
    {
        return "Hello, {$name}!";
    }
}

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Harness_Nexus_SyncSuccess')]
    public function run(string $endpoint)
    {
        /** @var TestService $service */
        $service = Workflow::newNexusServiceStub(
            TestService::class,
            NexusOperationOptions::new()
                ->withEndpoint($endpoint)
                ->withScheduleToCloseTimeout('1 minute'),
        );

        return yield $service->sayHello('world');
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(WorkflowClientInterface $client, Feature $feature): void
    {
        Assert::notNull($feature->nexusEndpoint, 'Nexus endpoint is not provided by the runner');

        $stub = $client->newUntypedWorkflowStub(
            'Harness_Nexus_SyncSuccess',
            WorkflowOptions::new()->withTaskQueue($feature->taskQueue),
        );
        $client->start($stub, $feature->nexusEndpoint);

        Assert::same($stub->getResult('string'), 'Hello, world!');

        $events = \iterator_to_array($client->getWorkflowHistory($stub->getExecution())->getEvents(), false);

        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationScheduledEventAttributes()),
            'NexusOperationScheduled event is missing',
        );
        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationCompletedEventAttributes()),
            'NexusOperationCompleted event is missing',
        );
        Assert::false(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationStartedEventAttributes()),
            'Synchronous operation must not produce a NexusOperationStarted event',
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
