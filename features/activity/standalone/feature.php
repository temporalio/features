<?php

declare(strict_types=1);

namespace Harness\Feature\Activity\Standalone;

use Harness\Attribute\Check;
use Harness\Runtime\Feature;
use Harness\Runtime\State;
use Ramsey\Uuid\Uuid;
use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Client\Activity\ActivityExecutionStatus;
use Temporal\Client\Activity\StartActivityOptions;
use Temporal\Client\ActivityClient;
use Temporal\Client\ClientOptions;
use Temporal\Client\GRPC\ServiceClientInterface;
use Webmozart\Assert\Assert;

#[ActivityInterface]
class FeatureActivity
{
    #[ActivityMethod('echo')]
    public function echo(string $value): string
    {
        return $value;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        ServiceClientInterface $serviceClient,
        Feature $feature,
        State $runtime,
    ): void {
        $client = ActivityClient::create(
            serviceClient: $serviceClient,
            options: (new ClientOptions())->withNamespace($runtime->namespace),
        );

        $handle = $client->start(
            'echo',
            StartActivityOptions::new()
                ->withId(Uuid::uuid4()->toString())
                ->withTaskQueue($feature->taskQueue)
                ->withStartToCloseTimeout('1 minute'),
            'hello',
        );

        Assert::same($handle->getResult('string'), 'hello');
        Assert::same($handle->describe()->info->status, ActivityExecutionStatus::Completed);
    }
}
