<?php

declare(strict_types=1);

namespace Harness\Feature;

use Harness\Attribute\Stub;
use Harness\Runtime\Feature;
use Psr\Container\ContainerInterface;
use Spiral\Core\Attribute\Proxy;
use Spiral\Core\Container\InjectorInterface;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Client\WorkflowStubInterface;

/**
 * @implements InjectorInterface<WorkflowStubInterface>
 */
final class WorkflowStubInjector implements InjectorInterface
{
    public function __construct(
        #[Proxy] private ContainerInterface $container,
    ) {
    }

    public function createInjection(
        \ReflectionClass $class,
        \ReflectionParameter|null|string $context = null,
    ): WorkflowStubInterface {
        if (!$context instanceof \ReflectionParameter) {
            throw new \InvalidArgumentException('Context is not clear.');
        }

        /** @var Stub|null $attribute */
        $attribute = ($context->getAttributes(Stub::class)[0] ?? null)?->newInstance();
        if ($attribute === null) {
            throw new \InvalidArgumentException(\sprintf('Attribute %s is not found.', Stub::class));
        }

        /** @var WorkflowClientInterface $client */
        $client = $this->container->get(WorkflowClientInterface::class);
        /** @var Feature $feature */
        $feature = $this->container->get(Feature::class);
        $options = WorkflowOptions::new()
            ->withTaskQueue($feature->taskQueue)
            ->withEagerStart($attribute->eagerStart);

        $stub = $client->newUntypedWorkflowStub(
            $attribute->type,
            $options,
        );
        $client->start($stub);

        return $stub;
    }
}