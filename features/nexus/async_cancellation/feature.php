<?php

declare(strict_types=1);

namespace Harness\Feature\Nexus\AsyncCancellation;

use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Temporal\Api\History\V1\HistoryEvent;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Exception\Failure\NexusOperationFailure;
use Temporal\Nexus\Attribute\AsyncOperation;
use Temporal\Nexus\Attribute\Service;
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
    #[AsyncOperation(name: 'block-forever', output: 'string')]
    public function blockForever(string $name): WorkflowHandle
    {
        return WorkflowHandle::fromWorkflowMethod(
            BlockingWorkflow::class,
            WorkflowOptions::new()->withWorkflowId('async-cancellation-' . $name),
            $name,
        );
    }
}

#[WorkflowInterface]
class BlockingWorkflow
{
    #[WorkflowMethod('AsyncCancellationBlockingWorkflow')]
    public function run(string $name)
    {
        yield Workflow::await(static fn(): bool => false);

        return '';
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

        /** @var NexusOperationHandle<string>|null $handle */
        $handle = null;
        $scope = Workflow::async(static function () use ($stub, &$handle) {
            $handle = yield $stub->start('block-forever', ['world'], 'string');
            yield $handle->getResult();
        });

        yield Workflow::await(static function () use (&$handle): bool {
            return $handle !== null;
        });
        yield Workflow::timer(CarbonInterval::seconds(1));
        $scope->cancel();

        try {
            yield $scope;
        } catch (CanceledFailure) {
            return 'canceled';
        } catch (NexusOperationFailure $e) {
            if ($e->getPrevious() instanceof CanceledFailure) {
                return 'canceled';
            }

            throw $e;
        }

        throw new \RuntimeException('expected the cancelled operation to fail');
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

        Assert::same($stub->getResult('string'), 'canceled');

        $events = \iterator_to_array($client->getWorkflowHistory($stub->getExecution())->getEvents(), false);

        Assert::true(
            self::hasEvent(
                $events,
                static fn(HistoryEvent $e): bool => $e->hasNexusOperationCancelRequestedEventAttributes(),
            ),
            'NexusOperationCancelRequested event is missing',
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
