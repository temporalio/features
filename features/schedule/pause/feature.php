<?php

declare(strict_types=1);

namespace Harness\Feature\Schedule\Pause;

use Carbon\CarbonInterval;
use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Temporal\Client\Schedule\Action\StartWorkflowAction;
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
        $handle = $client->createSchedule(
            schedule: Schedule::new()
                ->withAction(
                    StartWorkflowAction::new('Workflow')
                        ->withTaskQueue($feature->taskQueue)
                        ->withInput(['arg1'])
                )->withSpec(
                    ScheduleSpec::new()
                        ->withIntervalList(CarbonInterval::minute(1))
                )->withState(
                    ScheduleState::new()
                        ->withPaused(true)
                        ->withNotes('initial note')
                ),
            options: ScheduleOptions::new()
                ->withNamespace($runtime->namespace),
        );

        try {
            // Confirm pause
            $state = $handle->describe()->schedule->state;
            Assert::true($state->paused);
            Assert::same($state->notes, 'initial note');
            // Re-pause
            $handle->pause('custom note1');
            $state = $handle->describe()->schedule->state;
            Assert::true($state->paused);
            Assert::same($state->notes, 'custom note1');
            // Unpause
            $handle->unpause();
            $state = $handle->describe()->schedule->state;
            Assert::false($state->paused);
            Assert::same($state->notes, 'Unpaused via PHP SDK');
            // Pause
            $handle->pause();
            $state = $handle->describe()->schedule->state;
            Assert::true($state->paused);
            Assert::same($state->notes, 'Paused via PHP SDK');
        } finally {
            $handle->delete();
        }
    }
}
