<?php

declare(strict_types=1);

namespace Harness\Feature;

use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Psr\Container\ContainerInterface;
use Spiral\Core\Attribute\Proxy;
use Spiral\Core\Container\InjectorInterface;
use Spiral\Core\InvokerInterface;
use Temporal\Client\ClientOptions;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowClientInterface;
use Temporal\Client\WorkflowOptions;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Interceptor\GrpcClientInterceptor;
use Temporal\Interceptor\PipelineProvider;

/**
 * @implements InjectorInterface<WorkflowStubInterface>
 */
final class WorkflowStubInjector implements InjectorInterface
{
    public function __construct(
        #[Proxy] private readonly ContainerInterface $container,
        #[Proxy] private readonly InvokerInterface $invoker,
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
        $client = $this->getClient($context);

        /** @var Feature $feature */
        $feature = $this->container->get(Feature::class);
        $options = WorkflowOptions::new()
            ->withTaskQueue($feature->taskQueue)
            ->withEagerStart($attribute->eagerStart);

        $attribute->workflowId === null or $options = $options->withWorkflowId($attribute->workflowId);
        $attribute->memo === [] or $options = $options->withMemo($attribute->memo);

        $stub = $client->newUntypedWorkflowStub($attribute->type, $options);
        $client->start($stub, ...$attribute->args);

        return $stub;
    }

    public function getClient(\ReflectionParameter $context): WorkflowClientInterface
    {
        /** @var Client|null $attribute */
        $attribute = ($context->getAttributes(Client::class)[0] ?? null)?->newInstance();

        /** @var WorkflowClientInterface $client */
        $client = $this->container->get(WorkflowClientInterface::class);

        if ($attribute === null) {
            return $client;
        }

        // PipelineProvider is set
        if ($attribute->pipelineProvider !== null) {
            $provider = $this->invoker->invoke($attribute->pipelineProvider);
            \assert($provider instanceof PipelineProvider);

            // Build custom WorkflowClient with gRPC interceptor
            $serviceClient = $client->getServiceClient()
                ->withInterceptorPipeline($provider->getPipeline(GrpcClientInterceptor::class));

            /** @var State $runtime */
            $runtime = $this->container->get(State::class);

            $client = WorkflowClient::create(
                serviceClient: $serviceClient,
                options: (new ClientOptions())->withNamespace($runtime->namespace),
                interceptorProvider: $provider,
            )->withTimeout(5);
        }

        $attribute->timeout === null or $client = $client->withTimeout($attribute->timeout);

        return $client;
    }
}
