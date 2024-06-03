<?php

declare(strict_types=1);

namespace Harness\Feature\Query\UnexpectedQueryTypeName;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowQueryException;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private bool $beDone = false;

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        yield Workflow::await(fn(): bool => $this->beDone);
    }

    #[SignalMethod('finish')]
    public function finish(): void
    {
        $this->beDone = true;
    }
}

class FeatureChecker
{
    #[Check]
    public static function check(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        try {
            $stub->query('nonexistent');
            throw new \Exception('Query must fail due to unknown queryType');
        } catch (WorkflowQueryException $e) {
            Assert::contains(
                $e->getPrevious()->getMessage(),
                'unknown queryType nonexistent',
            );
        }

        $stub->signal('finish');
        $stub->getResult();
    }
}
