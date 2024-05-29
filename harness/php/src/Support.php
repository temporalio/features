<?php

declare(strict_types=1);

namespace Harness;

final class Support
{
    public static function echoException(\Throwable $e): void
    {
        $trace = \array_filter($e->getTrace(), static fn(array $trace): bool =>
            isset($trace['file']) &&
            !\str_contains($trace['file'], DIRECTORY_SEPARATOR . 'vendor' . DIRECTORY_SEPARATOR),
        );
        \array_pop($trace);

        foreach ($trace as $line) {
            echo "-> \e[1;33m{$line['file']}:{$line['line']}\e[0m\n";
        }

        do {
            /** @var \Throwable $err */
            $err = $e;
            $name = \substr(\strrchr($e::class, "\\"), 1);
            echo "\e[1;34m$name\e[0m\n";
            echo "\e[3m{$e->getMessage()}\e[0m\n";
            $e = $e->getPrevious();
        } while ($e !== null);
    }
}
