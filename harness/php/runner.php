<?php

declare(strict_types=1);

use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Harness\RuntimeBuilder;
use Harness\Support;
use Psr\Container\ContainerInterface;
use Spiral\Core\Scope;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClient;
use Temporal\Client\GRPC\ServiceClientInterface;
use Temporal\Client\ScheduleClient;
use Temporal\Client\ScheduleClientInterface;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;

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
echo "Connecting to Temporal service at {$runtime->address}... ";
try {
    $serviceClient->getConnection()->connect(5);
    echo "\e[1;32mOK\e[0m\n";
} catch (\Throwable $e) {
    echo "\e[1;31mFAILED\e[0m\n";
    Support::echoException($e);
    return;
}

// TODO if authKey is set
// $serviceClient->withAuthKey($authKey)

$workflowClient = WorkflowClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
)->withTimeout(5);

$scheduleClient = ScheduleClient::create(
    serviceClient: $serviceClient,
    options: (new ClientOptions())->withNamespace($runtime->namespace),
)->withTimeout(5);

$container = new Spiral\Core\Container();
$container->bindSingleton(State::class, $runtime);
$container->bindSingleton(ServiceClientInterface::class, $serviceClient);
$container->bindSingleton(WorkflowClientInterface::class, $workflowClient);
$container->bindSingleton(ScheduleClientInterface::class, $scheduleClient);

// Run checks
foreach ($runtime->checks() as $feature => $definition) {
    try {
        $container->runScope(
            new Scope(name: 'feature',bindings: [
                Feature::class => $feature,
            ]),
            static function (ContainerInterface $container) use ($definition) {
                // todo modify services based on feature requirements
                [$class, $method] = $definition;
                echo "Running check \e[1;36m{$class}::{$method}\e[0m ";
                $container->invoke($definition);
                echo "\e[1;32mOK\e[0m\n";
            },
        );
    } catch (\Throwable $e) {
        \trap($e);

        echo "\e[1;31mFAILED\e[0m\n";
        Support::echoException($e);
        echo "\n";
    }
}
