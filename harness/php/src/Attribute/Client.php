<?php

declare(strict_types=1);

namespace Harness\Attribute;

/**
 * An attribute to configure client options.
 *
 * @see \Harness\Feature\WorkflowStubInjector
 */
#[\Attribute(\Attribute::TARGET_PARAMETER)]
final class Client
{
    public function __construct(
        public int|string $timeout,
    ) {
    }
}
