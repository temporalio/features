<?php

declare(strict_types=1);

namespace Harness\Input;

final class Feature
{
    public function __construct(
        public string $dir,
        public string $namespace,
        public string $taskQueue,
    ) {
    }
}
