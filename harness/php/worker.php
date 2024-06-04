<?php

declare(strict_types=1);

use Harness\Runtime\State;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClient;
use Temporal\Client\GRPC\ServiceClientInterface;
use Temporal\Client\ScheduleClient;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Worker\WorkerInterface;
use Temporal\Worker\WorkerOptions;
use Temporal\WorkerFactory;

ini_set('display_errors', 'stderr');
include "vendor/autoload.php";

/** @var array<non-empty-string, WorkerInterface> $run */
$workers = [];
$factory = WorkerFactory::create();
$container = new Spiral\Core\Container();
$getWorker = static function (string $taskQueue) use (&$workers, $factory): WorkerInterface {
    return $workers[$taskQueue] ??= $factory->newWorker(
        $taskQueue,
        WorkerOptions::new()->withMaxConcurrentActivityExecutionSize(10)
    );
};

try {
    $runtime = \Harness\RuntimeBuilder::createState($argv, \dirname(__DIR__, 2) . '/features/');
    $run = $runtime->command;

    // Create client services
    $serviceClient = $runtime->command->tlsKey === null && $runtime->command->tlsCert === null
        ? ServiceClient::create($runtime->address)
        : ServiceClient::createSSL(
            $runtime->address,
            clientKey: $runtime->command->tlsKey,
            clientPem: $runtime->command->tlsCert,
        );
    $options = (new ClientOptions())->withNamespace($runtime->namespace);
    $workflowClient = WorkflowClient::create(serviceClient: $serviceClient, options: $options);
    $scheduleClient = ScheduleClient::create(serviceClient: $serviceClient, options: $options);

    // Bind services
    $container->bindSingleton(State::class, $runtime);
    $container->bindSingleton(ServiceClientInterface::class, $serviceClient);
    $container->bindSingleton(WorkflowClientInterface::class, $workflowClient);
    $container->bindSingleton(ScheduleClientInterface::class, $scheduleClient);

    // Register Workflows
    foreach ($runtime->workflows() as $feature => $workflow) {
        $getWorker($feature->taskQueue)->registerWorkflowTypes($workflow);
    }

    // Register Activities
    foreach ($runtime->activities() as $feature => $activity) {
        $getWorker($feature->taskQueue)->registerActivityImplementations($container->make($activity));
    }

    $factory->run();
} catch (\Throwable $e) {
    \td($e);
}
