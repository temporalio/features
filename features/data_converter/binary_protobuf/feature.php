<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\BinaryProtobuf;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Temporal\Api\Common\V1\DataBlob;
use Temporal\Api\Common\V1\Payload;
use Temporal\Api\Workflowservice\V1\StartWorkflowExecutionRequest;
use Temporal\Client\GRPC\ContextInterface;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\ProtoConverter;
use Temporal\Interceptor\GrpcClientInterceptor;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const EXPECTED_RESULT = 0xDEADBEEF;
\define(__NAMESPACE__ . '\INPUT', (new DataBlob())->setData(EXPECTED_RESULT));

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(DataBlob $data)
    {
        return $data;
    }
}

/**
 * Catches {@see StartWorkflowExecutionRequest} from the gRPC calls.
 */
class GrpcCallInterceptor implements GrpcClientInterceptor
{
    public ?StartWorkflowExecutionRequest $startRequest = null;

    public function interceptCall(string $method, object $arg, ContextInterface $ctx, callable $next): object
    {
        $arg instanceof StartWorkflowExecutionRequest and $this->startRequest = $arg;
        return $next($method, $arg, $ctx);
    }
}


class FeatureChecker
{
    public function __construct(
        private readonly GrpcCallInterceptor $interceptor = new GrpcCallInterceptor(),
    ) {}

    public function pipelineProvider(): PipelineProvider
    {
        return new SimplePipelineProvider([$this->interceptor]);
    }

    #[Check]
    public function check(
        #[Stub('Workflow', args: [INPUT])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [ProtoConverter::class],
        )]
        WorkflowStubInterface $stub,
    ): void {
        /** @var DataBlob $result */
        $result = $stub->getResult(DataBlob::class);

        # Check that binary protobuf message was decoded in the Workflow and sent back.
        # But we don't check the result Payload encoding, because we can't configure different Payload encoders
        # on the server side for different Harness features.
        # There `json/protobuf` converter is used for protobuf messages by default on the server side.
        Assert::eq($result->getData(), EXPECTED_RESULT);

        # Check arguments
        Assert::notNull($this->interceptor->startRequest);
        /** @var Payload $payload */
        $payload = $this->interceptor->startRequest->getInput()?->getPayloads()[0] ?? null;
        Assert::notNull($payload);

        Assert::same($payload->getMetadata()['encoding'], 'binary/protobuf');
        Assert::same($payload->getMetadata()['messageType'], 'temporal.api.common.v1.DataBlob');
    }
}
