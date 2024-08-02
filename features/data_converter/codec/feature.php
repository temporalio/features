<?php

declare(strict_types=1);

namespace Harness\Feature\DataConverter\Codec;

use Harness\Attribute\Check;
use Harness\Attribute\Client;
use Harness\Attribute\Stub;
use Temporal\Api\Common\V1\Payload;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\EncodedValues;
use Temporal\DataConverter\PayloadConverterInterface;
use Temporal\DataConverter\Type;
use Temporal\Interceptor\PipelineProvider;
use Temporal\Interceptor\SimplePipelineProvider;
use Temporal\Interceptor\Trait\WorkflowClientCallsInterceptorTrait;
use Temporal\Interceptor\WorkflowClient\GetResultInput;
use Temporal\Interceptor\WorkflowClient\StartInput;
use Temporal\Interceptor\WorkflowClientCallsInterceptor;
use Temporal\Workflow\WorkflowExecution;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const CODEC_ENCODING = 'my-encoding';
const EXPECTED_RESULT = new DTO(spec: true);

#[WorkflowInterface]
class FeatureWorkflow
{
    #[WorkflowMethod('Workflow')]
    public function run(mixed $data)
    {
        return $data;
    }
}

/**
 * Catches raw Workflow result and input.
 */
class ResultInterceptor implements WorkflowClientCallsInterceptor
{
    use WorkflowClientCallsInterceptorTrait;
    public ?EncodedValues $result = null;
    public ?EncodedValues $start = null;
    public function getResult(GetResultInput $input, callable $next): ?EncodedValues
    {
        return $this->result = $next($input);
    }

    public function start(StartInput $input, callable $next): WorkflowExecution
    {
        $this->start = $input->arguments;
        return $next($input);
    }
}

#[\AllowDynamicProperties]
class DTO
{
    public function __construct(...$args)
    {
        foreach ($args as $key => $value) {
            $this->{$key} = $value;
        }
    }
}

class Base64PayloadCodec implements PayloadConverterInterface
{
    public function getEncodingType(): string
    {
        return CODEC_ENCODING;
    }

    public function toPayload($value): ?Payload
    {
        return $value instanceof DTO
            ? (new Payload())
                ->setData(\base64_encode(\json_encode($value, flags: \JSON_THROW_ON_ERROR)))
                ->setMetadata(['encoding' => CODEC_ENCODING])
            : null;
    }

    public function fromPayload(Payload $payload, Type $type): DTO
    {
        $values = \json_decode(\base64_decode($payload->getData()), associative: true, flags: \JSON_THROW_ON_ERROR);
        $dto = new DTO();
        foreach ($values as $key => $value) {
            $dto->{$key} = $value;
        }
        return $dto;
    }
}

class FeatureChecker
{
    public function __construct(
        private ResultInterceptor $interceptor = new ResultInterceptor(),
    ) {}

    public function pipelineProvider(): PipelineProvider
    {
        return new SimplePipelineProvider([$this->interceptor]);
    }

    #[Check]
    public function check(
        #[Stub('Workflow', args: [EXPECTED_RESULT])]
        #[Client(
            pipelineProvider: [FeatureChecker::class, 'pipelineProvider'],
            payloadConverters: [Base64PayloadCodec::class]),
        ]
        WorkflowStubInterface $stub,
    ): void {
        $result = $stub->getResult();

        Assert::eq($result, EXPECTED_RESULT);

        $result = $this->interceptor->result;
        $input = $this->interceptor->start;
        Assert::notNull($result);
        Assert::notNull($input);

        // Check result value from interceptor
        /** @var Payload $resultPayload */
        $resultPayload = $result->toPayloads()->getPayloads()[0];
        Assert::same($resultPayload->getMetadata()['encoding'], CODEC_ENCODING);
        Assert::same($resultPayload->getData(), \base64_encode('{"spec":true}'));

        // Check arguments from interceptor
        /** @var Payload $inputPayload */
        $inputPayload = $input->toPayloads()->getPayloads()[0];
        Assert::same($inputPayload->getMetadata()['encoding'], CODEC_ENCODING);
        Assert::same($inputPayload->getData(), \base64_encode('{"spec":true}'));
    }
}
