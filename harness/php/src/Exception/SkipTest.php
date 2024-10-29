<?php

declare(strict_types=1);

namespace Harness\Exception;

final class SkipTest extends \RuntimeException
{
    public function __construct(
        public readonly string $reason,
    ) {
    }
}
