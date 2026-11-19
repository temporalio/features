<?php

declare(strict_types=1);

namespace Harness\Feature\ExternalStorage\S3Interop;

use Aws\S3\S3Client;
use Harness\Attribute\Check;
use Harness\Exception\SkipTest;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Temporal\Api\Common\V1\Payload;
use Temporal\Api\Common\V1\Payloads;
use Temporal\Api\Common\V1\WorkflowExecution;
use Temporal\Api\Sdk\V1\ExternalStorageReference;
use Temporal\Api\Workflowservice\V1\GetWorkflowExecutionHistoryRequest;
use Temporal\Api\Workflowservice\V1\SignalWorkflowExecutionRequest;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClientInterface;
use Temporal\Client\WorkflowClient;
use Temporal\Client\WorkflowOptions;
use Temporal\DataConverter\ExternalStorage\ExternalStorage;
use Temporal\DataConverter\ExternalStorage\S3\AwsS3Client;
use Temporal\DataConverter\ExternalStorage\S3\S3StorageDriver;
use Temporal\Exception\Failure\ApplicationFailure;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

const DRIVER_NAME = 'aws.s3driver';
const REFERENCE_TYPE = 'temporal.api.sdk.v1.ExternalStorageReference';
const SIGNAL = 'foreign';
const FOREIGN_VALUE = 'hello';
const FOREIGN_PAYLOAD = 'ChYKCGVuY29kaW5nEgpqc29uL3BsYWluEgciaGVsbG8i';
const FOREIGN_HASH = '1b8435174847de88db3a8c5a764295d62add152743b3a9a730796a4c090caf8c';
const FOREIGN_KEY = 'v0/d/sha256/' . FOREIGN_HASH;
const INPUT_HASH = '8016abfa3587d601e521edf45b84c3fca4ef12f10d6b470d4c4aa0eab0f82f1a';
\define(__NAMESPACE__ . '\INPUT', \str_repeat('x', 300 * 1024));

#[WorkflowInterface]
class FeatureWorkflow
{
    private ?string $foreign = null;

    #[WorkflowMethod('Workflow')]
    public function run(string $value)
    {
        yield Workflow::await(fn(): bool => $this->foreign !== null);
        if ($this->foreign !== FOREIGN_VALUE) {
            throw new ApplicationFailure(
                \sprintf('foreign payload is %s, expected %s', $this->foreign, FOREIGN_VALUE),
                'Mismatch',
                true,
            );
        }

        return $value;
    }

    #[SignalMethod(SIGNAL)]
    public function foreign(string $value): void
    {
        $this->foreign = $value;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(ServiceClientInterface $serviceClient, State $runtime, Feature $feature): void
    {
        $bucket = (string) \getenv('TEMPORAL_FEATURES_S3_BUCKET');
        if ($bucket === '') {
            throw new SkipTest('TEMPORAL_FEATURES_S3_BUCKET is not set');
        }

        $s3 = self::s3Client();
        $s3->putObject(['Bucket' => $bucket, 'Key' => FOREIGN_KEY, 'Body' => \base64_decode(FOREIGN_PAYLOAD)]);

        $client = WorkflowClient::create(
            $serviceClient,
            (new ClientOptions())
                ->withNamespace($runtime->namespace)
                ->withExternalStorage(new ExternalStorage([new S3StorageDriver(new AwsS3Client($s3), $bucket)])),
        );
        $stub = $client->newUntypedWorkflowStub(
            'Workflow',
            WorkflowOptions::new()
                ->withTaskQueue($feature->taskQueue)
                ->withWorkflowExecutionTimeout('1 minute'),
        );
        $client->start($stub, INPUT);
        $execution = new WorkflowExecution([
            'workflow_id' => $stub->getExecution()->getID(),
            'run_id' => (string) $stub->getExecution()->getRunID(),
        ]);

        $reference = new ExternalStorageReference([
            'driver_name' => DRIVER_NAME,
            'claim_data' => [
                'bucket' => $bucket,
                'key' => FOREIGN_KEY,
                'hash_algorithm' => 'sha256',
                'hash_value' => FOREIGN_HASH,
            ],
        ]);
        $serviceClient->SignalWorkflowExecution(
            (new SignalWorkflowExecutionRequest())
                ->setNamespace($runtime->namespace)
                ->setWorkflowExecution($execution)
                ->setSignalName(SIGNAL)
                ->setInput(new Payloads(['payloads' => [new Payload([
                    'metadata' => ['encoding' => 'json/protobuf', 'messageType' => REFERENCE_TYPE],
                    'data' => $reference->serializeToJsonString(),
                ])]])),
        );

        Assert::same($stub->getResult('string'), INPUT);

        $history = $serviceClient->GetWorkflowExecutionHistory(
            (new GetWorkflowExecutionHistoryRequest())
                ->setNamespace($runtime->namespace)
                ->setExecution($execution),
        )->getHistory();
        $started = $completed = null;
        foreach ($history->getEvents() as $event) {
            $started ??= $event->getWorkflowExecutionStartedEventAttributes();
            $completed ??= $event->getWorkflowExecutionCompletedEventAttributes();
        }
        Assert::notNull($started);
        Assert::notNull($completed);

        $prefix = \sprintf(
            'v0/ns/%s/wt/%s/wi/%s',
            $runtime->namespace,
            $started->getWorkflowType()->getName(),
            $execution->getWorkflowId(),
        );
        $inputKey = $prefix . '/ri/null/d/sha256/' . INPUT_HASH;
        self::checkReference($started->getInput()->getPayloads()[0], $bucket, $inputKey);
        self::checkReference(
            $completed->getResult()->getPayloads()[0],
            $bucket,
            $prefix . '/ri/' . $execution->getRunId() . '/d/sha256/' . INPUT_HASH,
        );

        $object = (string) $s3->getObject(['Bucket' => $bucket, 'Key' => $inputKey])['Body'];
        Assert::same(\hash('sha256', $object), INPUT_HASH);
    }

    private static function checkReference(Payload $payload, string $bucket, string $key): void
    {
        Assert::same($payload->getMetadata()['encoding'], 'json/protobuf');
        Assert::same($payload->getMetadata()['messageType'], REFERENCE_TYPE);
        Assert::count($payload->getExternalPayloads(), 1);

        $reference = new ExternalStorageReference();
        $reference->mergeFromJsonString($payload->getData(), true);
        Assert::same($reference->getDriverName(), DRIVER_NAME);
        $claimData = \iterator_to_array($reference->getClaimData());
        \ksort($claimData);
        Assert::same($claimData, [
            'bucket' => $bucket,
            'hash_algorithm' => 'sha256',
            'hash_value' => INPUT_HASH,
            'key' => $key,
        ]);
    }

    private static function s3Client(): S3Client
    {
        $config = [
            'region' => \getenv('AWS_REGION') ?: 'us-east-1',
            'version' => 'latest',
        ];
        $endpoint = (string) \getenv('TEMPORAL_FEATURES_S3_ENDPOINT');
        if ($endpoint !== '') {
            $config['endpoint'] = $endpoint;
            $config['use_path_style_endpoint'] = true;
        }

        return new S3Client($config);
    }
}
