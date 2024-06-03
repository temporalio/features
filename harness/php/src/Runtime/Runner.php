<?php

declare(strict_types=1);

namespace Harness\Runtime;

use Temporal\Testing\Environment;

final class Runner
{
    private Environment $environment;
    private bool $started = false;

    public function __construct(
        private State $runtime,
    ) {
        $this->environment = Environment::create();
        \register_shutdown_function(fn() => $this->stop());
    }

    public function start(): void
    {
        if ($this->started) {
            return;
        }

        $run = $this->runtime->command;
        $rrCommand = [
            './rr',
            'serve',
            '-o',
            "temporal.namespace={$this->runtime->namespace}",
            '-o',
            "temporal.address={$this->runtime->address}",
            '-o',
            'server.command=php,worker.php,' . \implode(',', $run->toCommandLineArguments()),
        ];
        $run->tlsKey === null or $rrCommand = [...$rrCommand, '-o', "tls.key={$run->tlsKey}"];
        $run->tlsCert === null or $rrCommand = [...$rrCommand, '-o', "tls.cert={$run->tlsCert}"];
        $command = \implode(' ', $rrCommand);

        echo "\e[1;36mStart RoadRunner with command:\e[0m {$command}\n";
        $this->environment->startRoadRunner($command);
        $this->started = true;
    }

    public function stop(): void
    {
        if (!$this->started) {
            return;
        }

        echo "\e[1;36mStop RoadRunner\e[0m\n";
        $this->environment->stop();
        $this->started = false;
    }
}