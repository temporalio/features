<?php

declare(strict_types=1);

namespace Harness\Feature\Activity\RetryOnError;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Activity;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Common\RetryOptions;
use Temporal\Exception\Client\WorkflowFailedException;
use Temporal\Exception\Failure\ActivityFailure;
use Temporal\Exception\Failure\ApplicationFailure;
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
        # Allow 4 retries with basically no backoff
        yield Workflow::newActivityStub(
            FeatureActivity::class,
            ActivityOptions::new()
                ->withScheduleToCloseTimeout('1 minute')
                ->withRetryOptions((new RetryOptions())
                    ->withInitialInterval('1 millisecond')
                    # Do not increase retry backoff each time
                    ->withBackoffCoefficient(1)
                    # 5 total maximum attempts
                    ->withMaximumAttempts(5)
                ),
        )->alwaysFailActivity();
    }
}

#[ActivityInterface]
class FeatureActivity
{
    #[ActivityMethod('always_fail_activity')]
    public function alwaysFailActivity(): string
    {
        $attempt = Activity::getInfo()->attempt;
        throw new ApplicationFailure(
            message: "activity attempt {$attempt} failed",
            type: "CustomError",
            nonRetryable: false,
        );
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(#[Stub('Workflow')] WorkflowStubInterface $stub): void
    {
        try {
            $stub->getResult();
            throw new \Exception('Expected WorkflowFailedException');
        } catch (WorkflowFailedException $e) {
            Assert::isInstanceOf($e->getPrevious(), ActivityFailure::class);
            /** @var ActivityFailure $failure */
            $failure = $e->getPrevious()->getPrevious();
            Assert::isInstanceOf($failure, ApplicationFailure::class);
            Assert::contains($failure->getOriginalMessage(), 'activity attempt 5 failed');
        }
    }
}
