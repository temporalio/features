<?php

declare(strict_types=1);

namespace Harness\Runtime;

use Symfony\Component\Process\Process;

final class Runner
{
    private ?Process $process = null;

    public function __construct(
        private State $runtime,
    ) {
        \register_shutdown_function(fn() => $this->stop());
    }

    public function start(): void
    {
        if ($this->process?->isRunning()) {
            return;
        }

        $run = $this->runtime->command;
        $rrCommand = [
            $this->runtime->workDir . DIRECTORY_SEPARATOR . 'rr',
            'serve',
            '-w',
            $this->runtime->workDir,
            '-o',
            "temporal.namespace={$this->runtime->namespace}",
            '-o',
            "temporal.address={$this->runtime->address}",
            '-o',
            'server.command=' . \implode(',', [
                'php',
                $this->runtime->sourceDir . DIRECTORY_SEPARATOR . 'worker.php',
                ...$run->toCommandLineArguments(),
            ]),
        ];
        $run->tlsKey === null or $rrCommand = [...$rrCommand, '-o', "temporal.tls.key={$run->tlsKey}"];
        $run->tlsCert === null or $rrCommand = [...$rrCommand, '-o', "temporal.tls.cert={$run->tlsCert}"];
        $run->tlsCaCert === null or $rrCommand = [...$rrCommand, '-o', "temporal.tls.ca-cert={$run->tlsCaCert}"];

        $this->process = new Process($rrCommand);
        $this->process->setTimeout(null);
        $this->process->start();
        \usleep(500_000);
    }

    public function stop(): void
    {
        if ($this->process?->isRunning()) {
            $this->process->stop();
            $this->process = null;
        }
    }
}
