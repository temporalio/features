<?php

declare(strict_types=1);

namespace Harness\Feature\Schedule\DuplicateError;

use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Ramsey\Uuid\Uuid;
use Temporal\Client\GRPC\StatusCode;
use Temporal\Client\Schedule\Action\StartWorkflowAction;
use Temporal\Client\Schedule\Schedule;
use Temporal\Client\Schedule\ScheduleOptions;
use Temporal\Client\Schedule\Spec\ScheduleSpec;
use Temporal\Client\Schedule\Spec\ScheduleState;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Exception\Client\ServiceClientException;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(): void
    {
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        ScheduleClientInterface $client,
        Feature $feature,
        State $runtime,
    ): void {
        $scheduleId = Uuid::uuid4()->toString();
        $schedule = Schedule::new()
            ->withAction(
                StartWorkflowAction::new('Workflow')
                    ->withTaskQueue($feature->taskQueue)
            )->withSpec(
                ScheduleSpec::new()
                    ->withIntervalList(CarbonInterval::hour(1))
            )->withState(
                ScheduleState::new()->withPaused(true)
            );

        $handle = $client->createSchedule(
            schedule: $schedule,
            options: ScheduleOptions::new()
                ->withNamespace($runtime->namespace),
            scheduleId: $scheduleId,
        );

        try {
            // Creating again with the same schedule ID should throw with ALREADY_EXISTS.
            $thrown = false;
            try {
                $client->createSchedule(
                    schedule: $schedule,
                    options: ScheduleOptions::new()
                        ->withNamespace($runtime->namespace),
                    scheduleId: $scheduleId,
                );
            } catch (ServiceClientException $e) {
                Assert::same($e->getCode(), StatusCode::ALREADY_EXISTS);
                $thrown = true;
            }

            Assert::true($thrown, 'Expected ServiceClientException with ALREADY_EXISTS');
        } finally {
            $handle->delete();
        }
    }
}
