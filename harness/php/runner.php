<?php

declare(strict_types=1);

use Harness\Run;

ini_set('display_errors', 'stderr');
chdir(__DIR__);
include "vendor/autoload.php";

$run = Run::fromCommandLine($argv);

// Build RR run command
$rrCommand = [
    './rr', 'serve',
    '-o', "temporal.namespace={$run->namespace}",
    '-o', "temporal.address={$run->address}",
    '-o', 'server.command=php,worker.php,' . \implode(',', $run->toCommandLineArguments()),
];
$run->tlsKey === null or $rrCommand = [...$rrCommand, '-o', "tls.key={$run->tlsKey}"];
$run->tlsCert === null or $rrCommand = [...$rrCommand, '-o', "tls.cert={$run->tlsCert}"];

$environment = \Temporal\Testing\Environment::create();
$command = \implode(' ', $rrCommand);

echo "\e[1;36mStart RoadRunner with command:\e[0m {$command}\n";

$environment->startRoadRunner($command);
\register_shutdown_function(static fn() => $environment->stop());

// Todo: run client code
