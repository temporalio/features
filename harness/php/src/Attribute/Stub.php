<?php

declare(strict_types=1);

namespace Harness\Attribute;

/**
 * An attribute to mark a method as a Feature checker.
 */
#[\Attribute(\Attribute::TARGET_PARAMETER)]
final class Stub
{
    public function __construct(
        public string $type,
        public bool $eagerStart = false,
    ) {
    }
}
