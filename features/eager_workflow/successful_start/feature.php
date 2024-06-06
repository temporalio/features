<?php

declare(strict_types=1);

namespace Harness\Feature\EagerWorkflow\SuccessfulStart;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Temporal\Api\Workflowservice\V1\StartWorkflowExecutionResponse;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ContextInterface;
use Temporal\Client\GRPC\ServiceClientInterface;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowOptions;
use Temporal\Interceptor\GrpcClientInterceptor;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

\define('EXPECTED_RESULT', 'Hello World');

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run()
    {
        return EXPECTED_RESULT;
    }
}

/**
 * Catches {@see StartWorkflowExecutionResponse} from the gRPC calls.
 */
class grpcCallInterceptor implements GrpcClientInterceptor
{
    public ?StartWorkflowExecutionResponse $lastResponse = null;

    public function interceptCall(string $method, object $arg, ContextInterface $ctx, callable $next): object
    {
        $result = $next($method, $arg, $ctx);
        $result instanceof StartWorkflowExecutionResponse and $this->lastResponse = $result;
        return $result;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        State $runtime,
        Feature $feature,
        ServiceClientInterface $serviceClient,
    ): void {
        $pipelineProvider = new SimplePipelineProvider([
            $interceptor = new grpcCallInterceptor(),
        ]);

        // Build custom WorkflowClient with gRPC interceptor
        $workflowClient = WorkflowClient::create(
            serviceClient: $serviceClient
                ->withInterceptorPipeline($pipelineProvider->getPipeline(GrpcClientInterceptor::class)),
            options: (new ClientOptions())->withNamespace($runtime->namespace),
        )->withTimeout(30);

        // Execute the Workflow in eager mode
        $stub = $workflowClient->newUntypedWorkflowStub(
            workflowType: 'Workflow',
            options: WorkflowOptions::new()->withEagerStart()->withTaskQueue($feature->taskQueue),
        );
        $workflowClient->start($stub);

        // Check the result and the eager workflow proof
        Assert::same($stub->getResult(), EXPECTED_RESULT);
        Assert::notNull($interceptor->lastResponse);
        Assert::notNull($interceptor->lastResponse->getEagerWorkflowTask());
    }
}
