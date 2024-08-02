<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\Binary;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Temporal\Api\Common\V1\DataBlob;
use Temporal\Api\Common\V1\Payload;
use Temporal\Api\Workflowservice\V1\StartWorkflowExecutionRequest;
use Temporal\Client\GRPC\ContextInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\Bytes;
use Temporal\DataConverter\EncodedValues;
use Temporal\Interceptor\GrpcClientInterceptor;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Interceptor\Trait\WorkflowClientCallsInterceptorTrait;
use Temporal\Interceptor\WorkflowClient\GetResultInput;
use Temporal\Interceptor\WorkflowClientCallsInterceptor;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const CODEC_ENCODING = 'binary/plain';
\define(__NAMESPACE__ . '\EXPECTED_RESULT', (string)0xDEADBEEF);
\define(__NAMESPACE__ . '\INPUT', new Bytes(EXPECTED_RESULT));

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(Bytes $data)
    {
        return $data;
    }
}

class Interceptor implements GrpcClientInterceptor, WorkflowClientCallsInterceptor
{
    use WorkflowClientCallsInterceptorTrait;

    public ?StartWorkflowExecutionRequest $startRequest = null;
    public ?EncodedValues $result = null;

    public function interceptCall(string $method, object $arg, ContextInterface $ctx, callable $next): object
    {
        $arg instanceof StartWorkflowExecutionRequest and $this->startRequest = $arg;
        return $next($method, $arg, $ctx);
    }

    public function getResult(GetResultInput $input, callable $next): ?EncodedValues
    {
        return $this->result = $next($input);
    }
}

class FeatureChecker
{
    public function __construct(
        private readonly Interceptor $interceptor = new Interceptor(),
    ) {}

    public function pipelineProvider(): PipelineProvider
    {
        return new SimplePipelineProvider([$this->interceptor]);
    }

    #[Check]
    public function check(
        #[Stub('Workflow', args: [INPUT])]
        #[Client(pipelineProvider: [FeatureChecker::class, 'pipelineProvider'])]
        WorkflowStubInterface $stub,
    ): void {
        /** @var Bytes $result */
        $result = $stub->getResult(Bytes::class);

        Assert::eq($result->getData(), EXPECTED_RESULT);

        # Check arguments
        Assert::notNull($this->interceptor->startRequest);
        Assert::notNull($this->interceptor->result);

        /** @var Payload $payload */
        $payload = $this->interceptor->startRequest->getInput()?->getPayloads()[0] ?? null;
        Assert::notNull($payload);

        Assert::same($payload->getMetadata()['encoding'], CODEC_ENCODING);

        // Check result value from interceptor
        /** @var Payload $resultPayload */
        $resultPayload = $this->interceptor->result->toPayloads()->getPayloads()[0];
        Assert::same($resultPayload->getMetadata()['encoding'], CODEC_ENCODING);
    }
}
