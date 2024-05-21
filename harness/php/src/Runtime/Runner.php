<?php

declare(strict_types=1);

namespace Harness\Runtime;

use Temporal\Testing\Environment;

final class Runner
{
    public static function runRoadRunner(State $runtime): Environment
    {
        $run = $runtime->command;
        $rrCommand = [
            './rr',
            'serve',
            '-o',
            "temporal.namespace={$runtime->namespace}",
            '-o',
            "temporal.address={$runtime->address}",
            '-o',
            'server.command=php,worker.php,' . \implode(',', $run->toCommandLineArguments()),
        ];
        $run->tlsKey === null or $rrCommand = [...$rrCommand, '-o', "tls.key={$run->tlsKey}"];
        $run->tlsCert === null or $rrCommand = [...$rrCommand, '-o', "tls.cert={$run->tlsCert}"];
        $environment = \Temporal\Testing\Environment::create();
        $command = \implode(' ', $rrCommand);

        echo "\e[1;36mStart RoadRunner with command:\e[0m {$command}\n";
        $environment->startRoadRunner($command);

        return $environment;
    }
}