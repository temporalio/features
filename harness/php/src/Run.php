<?php

declare(strict_types=1);

namespace Harness;

final class Run
{
    public string $namespace;

    /** @var list<Feature> */
    public array $features = [];

    public static function fromCommandLine(array $argv): self
    {
        $self = new self();
        foreach ($argv as $chunk) {
            if (\str_ends_with($chunk, '.php')) {
                continue;
            }

            if (\str_starts_with($chunk, 'namespace=')) {
                $self->namespace = \substr($chunk, 10);
                continue;
            }

            if (!\str_contains($chunk, ':')) {
                continue;
            }

            [$dir, $taskQueue] = \explode(':', $chunk, 2);
            $self->features[] = new Feature($dir, 'Harness\\Feature\\' . self::namespaceFromPath($dir), $taskQueue);
        }

        return $self;
    }

    private static function namespaceFromPath(string $dir): string
    {
        $normalized = \str_replace('/', '\\', \trim($dir, '/\\')) . '\\';
        // snake_case to PascalCase:
        return \str_replace('_', '', \ucwords($normalized, '_\\'));
    }
}
