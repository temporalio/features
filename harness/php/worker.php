<?php

declare(strict_types=1);

use Harness\ClassLocator;
use Harness\Run;
use Temporal\Activity\ActivityInterface;
use Temporal\Worker\WorkerInterface;
use Temporal\Worker\WorkerOptions;
use Temporal\WorkerFactory;
use Temporal\Workflow\WorkflowInterface;

ini_set('display_errors', 'stderr');
include "vendor/autoload.php";

$run = Run::fromCommandLine($argv);

/** @var array<non-empty-string, WorkerInterface> $run */
$workers = [];
$factory = WorkerFactory::create();
$getWorker = static function (string $taskQueue) use (&$workers, $factory): WorkerInterface {
    return $workers[$taskQueue] ??= $factory->newWorker(
        $taskQueue,
        WorkerOptions::new()->withMaxConcurrentActivityExecutionSize(10)
    );
};

$featuresDir = \dirname(__DIR__, 2) . '/features/';
foreach ($run->features as $feature) {
    foreach (ClassLocator::loadClasses($featuresDir . $feature->dir, $feature->namespace) as $class) {
        # Register Workflow
        $reflection = new \ReflectionClass($class);
        $attrs = $reflection->getAttributes(WorkflowInterface::class);
        if ($attrs !== []) {
            $getWorker($feature->taskQueue)->registerWorkflowTypes($class);
            continue;
        }

        # Register Activity
        $attrs = $reflection->getAttributes(ActivityInterface::class);
        if ($attrs !== []) {
            $getWorker($feature->taskQueue)->registerActivityImplementations(new $class());
        }
    }
}

$factory->run();
