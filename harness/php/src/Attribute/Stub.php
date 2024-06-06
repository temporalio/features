<?php

declare(strict_types=1);

namespace Harness\Attribute;

/**
 * An attribute to mark a method as a Feature checker.
 */
#[\Attribute(\Attribute::TARGET_PARAMETER)]
final class Stub
{
    /**
     * @param non-empty-string $type Workflow type.
     * @param non-empty-string|null $workflowId
     * @param list<mixed> $args
     */
    public function __construct(
        public string $type,
        public bool $eagerStart = false,
        public ?string $workflowId = null,
        public array $args = [],
        public array $memo = [],
    ) {
    }
}
