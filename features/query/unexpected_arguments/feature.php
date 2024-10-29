<?php

declare(strict_types=1);

namespace Harness\Feature\Query\UnexpectedArguments;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowQueryException;
use Temporal\Workflow;
use Temporal\Workflow\QueryMethod;
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

    #[QueryMethod('the_query')]
    public function theQuery(int $arg): string
    {
        return "got $arg";
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
        Assert::same('got 42', $stub->query('the_query', 42)?->getValue(0));

        try {
            $stub->query('the_query', true)?->getValue(0);
            throw new \Exception('Query must fail due to unexpected argument type');
        } catch (WorkflowQueryException $e) {
            Assert::contains(
                $e->getPrevious()->getMessage(),
                'The passed value of type "bool" can not be converted to required type "int"',
            );
        }

        # Silently drops extra arg
        Assert::same('got 123', $stub->query('the_query', 123, true)?->getValue(0));

        # Not enough arg
        try {
            $stub->query('the_query')?->getValue(0);
            throw new \Exception('Query must fail due to missing argument');
        } catch (WorkflowQueryException $e) {
            Assert::contains($e->getPrevious()->getMessage(), '0 passed and exactly 1 expected');
        }

        $stub->signal('finish');
        $stub->getResult();
    }
}
