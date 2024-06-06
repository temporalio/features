<?php

declare(strict_types=1);

namespace Harness\Feature\EagerWorkflow\SuccessfulStart;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Temporal\Api\Workflowservice\V1\StartWorkflowExecutionResponse;
use Temporal\Client\GRPC\ContextInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Interceptor\GrpcClientInterceptor;
use Temporal\Interceptor\PipelineProvider;
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
    private grpcCallInterceptor $interceptor;

    public function __construct()
    {
        $this->interceptor = new grpcCallInterceptor();
    }

    public function pipelineProvider(): PipelineProvider
    {
        return new SimplePipelineProvider([$this->interceptor]);
    }

    #[Check]
    public function check(
        #[Stub('Workflow', eagerStart: true, )]
        #[Client(timeout:30, pipelineProvider: [FeatureChecker::class, 'pipelineProvider'])]
        WorkflowStubInterface $stub,
    ): void {
        // Check the result and the eager workflow proof
        Assert::same($stub->getResult(), EXPECTED_RESULT);
        Assert::notNull($this->interceptor->lastResponse);
        Assert::notNull($this->interceptor->lastResponse->getEagerWorkflowTask());
    }
}
