<?php

declare(strict_types=1);

use Temporal\Worker\WorkerInterface;
use Temporal\Worker\WorkerOptions;
use Temporal\WorkerFactory;

ini_set('display_errors', 'stderr');
include "vendor/autoload.php";

/** @var array<non-empty-string, WorkerInterface> $run */
$workers = [];
$factory = WorkerFactory::create();
$getWorker = static function (string $taskQueue) use (&$workers, $factory): WorkerInterface {
    return $workers[$taskQueue] ??= $factory->newWorker(
        $taskQueue,
        WorkerOptions::new()->withMaxConcurrentActivityExecutionSize(10)
    );
};

try {
    $runtime = \Harness\RuntimeBuilder::createState($argv, \dirname(__DIR__, 2) . '/features/');
    $run = $runtime->command;

    // Register Workflows
    foreach ($runtime->workflows() as $feature => $workflow) {
        $getWorker($feature->taskQueue)->registerWorkflowTypes($workflow);
    }

    // Register Activities
    foreach ($runtime->activities() as $feature => $activity) {
        $getWorker($feature->taskQueue)->registerActivityImplementations(new $activity());
    }

    $factory->run();
} catch (\Throwable $e) {
    \trap($e);
    exit(1);
}