<?php

declare(strict_types=1);

namespace Harness\Feature\Update\BasicAsync;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowUpdateException;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private string $state = '';

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->state !== '');
        return $this->state;
    }

    #[Workflow\UpdateMethod('my_update')]
    public function myUpdate(string $arg): string
    {
        $this->state = $arg;
        return 'update-result';
    }

    #[Workflow\UpdateValidatorMethod('my_update')]
    public function myValidateUpdate(string $arg): void
    {
        $arg === 'bad-update-arg' and throw new \Exception('Invalid Update argument');
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        try {
            $stub->update('my_update', 'bad-update-arg');
            Assert::fail('Expected validation exception');
        } catch (WorkflowUpdateException $e) {
            Assert::contains($e->getPrevious()?->getMessage(), 'Invalid Update argument');
        }

        $updated = $stub->update('my_update', 'foo-bar')->getValue(0);
        Assert::same($updated, 'update-result');
        Assert::same($stub->getResult(), 'foo-bar');
    }
}
