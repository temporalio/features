<?php

declare(strict_types=1);

use Harness\RuntimeBuilder;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClient;
use Temporal\Client\ScheduleClient;
use Temporal\Client\WorkflowClient;

ini_set('display_errors', 'stderr');
chdir(__DIR__);
include "vendor/autoload.php";

$runtime = RuntimeBuilder::createState($argv, \dirname(__DIR__, 2) . '/features/');

// Run RoadRunner server if workflows or activities are defined
if (\iterator_to_array($runtime->workflows(), false) !== [] || \iterator_to_array($runtime->activities(), false) !== []) {
    $environment = \Harness\Runtime\Runner::runRoadRunner($runtime);
    \register_shutdown_function(static fn() => $environment->stop());
}

// Prepare and run checks

// Prepare services to be injected

$serviceClient = $runtime->command->tlsKey === null && $runtime->command->tlsCert === null
    ? ServiceClient::create($runtime->address)
    : ServiceClient::createSSL(
        $runtime->address,
        clientKey: $runtime->command->tlsKey,
        clientPem: $runtime->command->tlsCert,
    );
// TODO if authKey is set
// $serviceClient->withAuthKey($authKey)

$workflowClient = WorkflowClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
)->withTimeout(10); // default timeout 10s

$scheduleClient = ScheduleClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
)->withTimeout(10); // default timeout 10s

$arguments = [$serviceClient, $workflowClient, $scheduleClient];
$injector = new Yiisoft\Injector\Injector();

// Run checks
try {
    foreach ($runtime->checks() as $feature => $definition) {
        // todo modify services based on feature requirements
        [$class, $method] = $definition;
        echo "Running check \e[1;36m{$class}::{$method}\e[0m\n";
        $check = $injector->make($class, $arguments);
        $injector->invoke([$class, $method], $arguments);
    }
} catch (\Throwable $e) {
    \trap($e);
    exit(1);
}