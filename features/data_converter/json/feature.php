<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\Json;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
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

\define('EXPECTED_RESULT', (object)['spec' => true]);

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(object $data)
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
        #[Stub('Workflow', args: [EXPECTED_RESULT])]
        #[Client(pipelineProvider: [FeatureChecker::class, 'pipelineProvider'])]
        WorkflowStubInterface $stub,
    ): void {
        $result = $stub->getResult();

        Assert::eq($result, EXPECTED_RESULT);

        $result = $this->interceptor->result;
        Assert::notNull($result);

        $payloads = $result->toPayloads();
        /** @var \Temporal\Api\Common\V1\Payload $payload */
        $payload = $payloads->getPayloads()[0];

        Assert::same($payload->getMetadata()['encoding'], 'json/plain');
    }
}
