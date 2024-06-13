<?php

declare(strict_types=1);

namespace Harness\Feature\Schedule\Backfill;

use Carbon\CarbonImmutable;
use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Ramsey\Uuid\Uuid;
use Temporal\Client\Schedule\Action\StartWorkflowAction;
use Temporal\Client\Schedule\BackfillPeriod;
use Temporal\Client\Schedule\Policy\ScheduleOverlapPolicy;
use Temporal\Client\Schedule\Schedule;
use Temporal\Client\Schedule\ScheduleOptions;
use Temporal\Client\Schedule\Spec\ScheduleSpec;
use Temporal\Client\Schedule\Spec\ScheduleState;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(string $arg)
    {
        return $arg;
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
        $workflowId = Uuid::uuid4()->toString();
        $scheduleId = Uuid::uuid4()->toString();

        $handle = $client->createSchedule(
            schedule: Schedule::new()
                ->withAction(
                    StartWorkflowAction::new('Workflow')
                        ->withWorkflowId($workflowId)
                        ->withTaskQueue($feature->taskQueue)
                        ->withWorkflowId('arg1')
                )->withSpec(
                    ScheduleSpec::new()
                        ->withIntervalList(CarbonInterval::minute(1))
                )->withState(
                    ScheduleState::new()
                        ->withPaused(true)
                ),
            options: ScheduleOptions::new()
                ->withNamespace($runtime->namespace),
            scheduleId: $scheduleId,
        );

        // Run backfill
        $now = CarbonImmutable::now()->setSeconds(0);
        $threeYearsAgo = $now->modify('-3 years');
        $thirtyMinutesAgo = $now->modify('-30 minutes');
        $handle->backfill([
            BackfillPeriod::new(
                $threeYearsAgo->modify('-2 minutes'),
                $threeYearsAgo,
                ScheduleOverlapPolicy::AllowAll,
            ),
            BackfillPeriod::new(
                $thirtyMinutesAgo->modify('-2 minutes'),
                $thirtyMinutesAgo,
                ScheduleOverlapPolicy::AllowAll,
            ),
        ]);

        // Confirm 6 executions
        Assert::same($handle->describe()->info->numActions, 6);
    }
}
