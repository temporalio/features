<?php

declare(strict_types=1);

namespace Harness\Feature\Nexus\AsyncSuccess;

use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Nexus\Attribute\AsyncOperation;
use Temporal\Nexus\Attribute\Service;
use Temporal\Nexus\Nexus;
use Temporal\Nexus\WorkflowHandle;
use Temporal\Workflow;
use Temporal\Workflow\NexusOperationHandle;
use Temporal\Workflow\NexusOperationOptions;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[Service(name: 'test-service')]
class TestService
{
    #[AsyncOperation(name: 'say-hello-async', output: 'string')]
    public function sayHelloAsync(string $name): WorkflowHandle
    {
        return WorkflowHandle::fromWorkflowMethod(
            HandlerWorkflow::class,
            WorkflowOptions::new()->withWorkflowId('async-success-' . $name),
            $name,
        );
    }
}

#[WorkflowInterface]
class HandlerWorkflow
{
    #[WorkflowMethod('AsyncSuccessHandlerWorkflow')]
    public function run(string $name)
    {
        yield Workflow::timer(CarbonInterval::milliseconds(50));

        return "Hello, {$name}!";
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

        /** @var NexusOperationHandle<string> $handle */
        $handle = yield $stub->start('say-hello-async', ['world'], 'string');

        $token = $handle->getOperationToken();
        if ($token === null || $token === '') {
            throw new \RuntimeException('expected a non-empty operation token');
        }

        return 'token+' . (yield $handle->getResult());
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

        Assert::same($stub->getResult('string'), 'token+Hello, world!');

        $events = \iterator_to_array($client->getWorkflowHistory($stub->getExecution())->getEvents(), false);

        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationScheduledEventAttributes()),
            'NexusOperationScheduled event is missing',
        );
        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationStartedEventAttributes()),
            'NexusOperationStarted event is missing',
        );
        Assert::true(
            self::hasEvent($events, static fn(HistoryEvent $e): bool => $e->hasNexusOperationCompletedEventAttributes()),
            'NexusOperationCompleted event is missing',
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
