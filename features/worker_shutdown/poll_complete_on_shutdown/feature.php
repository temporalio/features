<?php

declare(strict_types=1);

namespace Harness\Feature\WorkerShutdown\PollCompleteOnShutdown;

use Harness\Attribute\Check;
use Harness\Exception\SkipTest;
use Harness\Runtime\Feature;
use Harness\Runtime\Runner;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Api\Enums\V1\EventType;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Common\RetryOptions;
use Temporal\Common\Uuid;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run()
    {
        $activity = Workflow::newActivityStub(
            FeatureActivity::class,
            ActivityOptions::new()
                ->withScheduleToCloseTimeout('10 seconds')
                ->withStartToCloseTimeout('5 seconds')
                ->withRetryOptions(RetryOptions::new()->withMaximumAttempts(1)),
        );

        while (true) {
            yield Workflow::timer(0.02);
            yield $activity->noop();
        }
    }
}

#[ActivityInterface]
class FeatureActivity
{
    #[ActivityMethod('noop')]
    public function noop(): void {}
}

class FeatureChecker
{
    private const WORKFLOW_COUNT = 5;
    private const SHUTDOWN_TIMEOUT_SECONDS = 5;
    private const HISTORY_TIMEOUT_SECONDS = 15;
    private const POLL_INTERVAL_MICROSECONDS = 200_000;

    #[Check]
    public static function check(
        WorkflowClientInterface $client,
        Runner $runner,
        Feature $feature,
    ): void {
        $pollCompleteOnShutdown = self::pollCompleteOnShutdownCapability();

        $stubs = [];
        for ($i = 0; $i < self::WORKFLOW_COUNT; $i++) {
            $stub = $client->newUntypedWorkflowStub(
                'Workflow',
                WorkflowOptions::new()
                    ->withWorkflowId('poll_complete_on_shutdown-' . Uuid::v4())
                    ->withTaskQueue($feature->taskQueue)
                    ->withWorkflowExecutionTimeout('1 minute')
                    ->withWorkflowTaskTimeout('5 seconds'),
            );
            $client->start($stub);
            $stubs[] = $stub;
        }

        try {
            foreach ($stubs as $stub) {
                self::waitForActivityScheduled($client, $stub);
            }

            $startedAt = \microtime(true);
            $runner->stop();
            $elapsed = \microtime(true) - $startedAt;

            Assert::lessThanEq(
                $elapsed,
                self::SHUTDOWN_TIMEOUT_SECONDS,
                \sprintf('Worker shutdown took %.2fs, expected at most %ds', $elapsed, self::SHUTDOWN_TIMEOUT_SECONDS),
            );

            $pollCompleteOnShutdown
                ? self::assertNoWorkflowTaskProblems($client, $stubs)
                : self::waitForAnyWorkflowTaskProblem($client, $stubs);
        } finally {
            foreach ($stubs as $stub) {
                $stub->terminate('feature cleanup');
            }

            $runner->start();
        }
    }

    private static function pollCompleteOnShutdownCapability(): bool
    {
        $capabilities = \getenv('FEATURE_NAMESPACE_CAPABILITIES');
        if ($capabilities === false || $capabilities === '') {
            throw new SkipTest('FEATURE_NAMESPACE_CAPABILITIES is not set');
        }

        $decoded = \json_decode($capabilities, true, flags: \JSON_THROW_ON_ERROR);
        Assert::isArray($decoded, 'FEATURE_NAMESPACE_CAPABILITIES must be a JSON object');
        Assert::keyExists($decoded, 'workerPollCompleteOnShutdown');

        return (bool) $decoded['workerPollCompleteOnShutdown'];
    }

    private static function waitForActivityScheduled(WorkflowClientInterface $client, WorkflowStubInterface $stub): void
    {
        $deadline = \microtime(true) + 10;

        while (\microtime(true) < $deadline) {
            if (self::hasEvent($client, $stub, [EventType::EVENT_TYPE_ACTIVITY_TASK_SCHEDULED])) {
                return;
            }

            \usleep(self::POLL_INTERVAL_MICROSECONDS);
        }

        throw new \RuntimeException("No activity was scheduled in {$stub->getExecution()->getID()}");
    }

    /**
     * @param list<WorkflowStubInterface> $stubs
     */
    private static function assertNoWorkflowTaskProblems(WorkflowClientInterface $client, array $stubs): void
    {
        foreach ($stubs as $stub) {
            Assert::false(
                self::hasWorkflowTaskProblem($client, $stub),
                "Unexpected workflow task problem in {$stub->getExecution()->getID()}",
            );
        }
    }

    /**
     * @param list<WorkflowStubInterface> $stubs
     */
    private static function waitForAnyWorkflowTaskProblem(WorkflowClientInterface $client, array $stubs): void
    {
        $deadline = \microtime(true) + self::HISTORY_TIMEOUT_SECONDS;

        while (\microtime(true) < $deadline) {
            foreach ($stubs as $stub) {
                if (self::hasWorkflowTaskProblem($client, $stub)) {
                    return;
                }
            }

            \usleep(self::POLL_INTERVAL_MICROSECONDS);
        }

        throw new \RuntimeException(
            'Expected a workflow task failure or timeout within ' . self::HISTORY_TIMEOUT_SECONDS . 's',
        );
    }

    private static function hasWorkflowTaskProblem(WorkflowClientInterface $client, WorkflowStubInterface $stub): bool
    {
        return self::hasEvent($client, $stub, [
            EventType::EVENT_TYPE_WORKFLOW_TASK_FAILED,
            EventType::EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT,
        ]);
    }

    /**
     * @param list<int> $types
     */
    private static function hasEvent(WorkflowClientInterface $client, WorkflowStubInterface $stub, array $types): bool
    {
        foreach ($client->getWorkflowHistory($stub->getExecution()) as $event) {
            if (\in_array($event->getEventType(), $types, true)) {
                return true;
            }
        }

        return false;
    }
}
