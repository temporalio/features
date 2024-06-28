<?php

declare(strict_types=1);

namespace Harness\Feature\Activity\CancelTryCancel;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use React\Promise\PromiseInterface;
use Temporal\Activity;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Common\RetryOptions;
use Temporal\Exception\Client\ActivityCanceledException;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private string $result = '';

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        # Start workflow
        $activity = Workflow::newActivityStub(
            FeatureActivity::class,
            ActivityOptions::new()
                ->withScheduleToCloseTimeout('1 minute')
                ->withHeartbeatTimeout('5 seconds')
                # Disable retry
                ->withRetryOptions(RetryOptions::new()->withMaximumAttempts(1))
                ->withCancellationType(Activity\ActivityCancellationType::TryCancel)
        );

        $scope = Workflow::async(static fn () => $activity->cancellableActivity());

        # Sleep for short time (force task turnover)
        yield Workflow::timer(1);

        try {
            $scope->cancel();
            yield $scope;
        } catch (CanceledFailure) {
            # Expected
        }

        # Wait for activity result
        yield Workflow::awaitWithTimeout('5 seconds', fn () => $this->result !== '');

        return $this->result;
    }

    #[Workflow\SignalMethod('activity_result')]
    public function activityResult(string $result)
    {
        $this->result = $result;
    }
}

#[ActivityInterface]
class FeatureActivity
{
    public function __construct(
        private readonly WorkflowClientInterface $client,
    ) {}

    /**
     * @return PromiseInterface<null>
     */
    #[ActivityMethod('cancellable_activity')]
    public function cancellableActivity()
    {
        # Heartbeat every second for a minute
        $result = 'timeout';
        try {
            for ($i = 0; $i < 5_0; $i++) {
                \usleep(100_000);
                Activity::heartbeat($i);
            }
        } catch (ActivityCanceledException $e) {
            $result = 'cancelled';
        } catch (\Throwable $e) {
            $result = 'unexpected';
        }

        # Send result as signal to workflow
        $execution = Activity::getInfo()->workflowExecution;
        $this->client
            ->newRunningWorkflowStub(FeatureWorkflow::class, $execution->getID(), $execution->getRunID())
            ->activityResult($result);
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(#[Stub('Workflow')] WorkflowStubInterface $stub): void
    {
        Assert::same($stub->getResult(timeout: 10), 'cancelled');
    }
}
