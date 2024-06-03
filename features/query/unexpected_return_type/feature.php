<?php

declare(strict_types=1);

namespace Harness\Feature\Query\UnexpectedReturnType;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\DataConverterException;
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
    public function theQuery(): string
    {
        return 'hi bob';
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
            $stub->query('the_query')?->getValue(0, 'int');
            throw new \Exception('Query must fail due to unexpected return type');
        } catch (DataConverterException $e) {
            Assert::contains(
                $e->getMessage(),
                'The passed value of type "string" can not be converted to required type "int"',
            );
        }

        $stub->signal('finish');
        $stub->getResult();
    }
}
