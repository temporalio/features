<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\JsonProtobuf;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Temporal\Api\Common\V1\DataBlob;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\EncodedValues;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Interceptor\Trait\WorkflowClientCallsInterceptorTrait;
use Temporal\Interceptor\WorkflowClient\GetResultInput;
use Temporal\Interceptor\WorkflowClientCallsInterceptor;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

\define('EXPECTED_RESULT', 0xDEADBEEF);
\define('INPUT', (new DataBlob())->setData(EXPECTED_RESULT));

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
 * Catches raw Workflow result.
 */
class ResultInterceptor implements WorkflowClientCallsInterceptor
{
    use WorkflowClientCallsInterceptorTrait;

    public ?EncodedValues $result = null;

    public function getResult(GetResultInput $input, callable $next): ?EncodedValues
    {
        return $this->result = $next($input);
    }
}

class FeatureChecker
{
    private ResultInterceptor $interceptor;

    public function __construct()
    {
        $this->interceptor = new ResultInterceptor();
    }

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
        /** @var DataBlob $result */
        $result = $stub->getResult(DataBlob::class);

        Assert::eq($result->getData(), EXPECTED_RESULT);

        $result = $this->interceptor->result;
        Assert::notNull($result);

        $payloads = $result->toPayloads();
        /** @var \Temporal\Api\Common\V1\Payload $payload */
        $payload = $payloads->getPayloads()[0];

        Assert::same($payload->getMetadata()['encoding'], 'json/protobuf');
        Assert::same($payload->getMetadata()['messageType'], 'temporal.api.common.v1.DataBlob');
        Assert::same($payload->getData(), '{"data":"MzczNTkyODU1OQ=="}');
    }
}
