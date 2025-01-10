<?php

declare(strict_types=1);

namespace Harness\Feature\ChildWorkflow\ThrowsOnExecute;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Temporal\Client\WorkflowStubInterface;
use Temporal\DataConverter\EncodedValues;
use Temporal\Exception\Client\WorkflowFailedException;
use Temporal\Exception\Failure\ApplicationFailure;
use Temporal\Exception\Failure\ChildWorkflowFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class MainWorkflow
{
    #[WorkflowMethod('MainWorkflow')]
    public function run()
    {
        return yield Workflow::newChildWorkflowStub(
            ChildWorkflow::class,
            // TODO: remove after https://github.com/temporalio/sdk-php/issues/451 is fixed
            Workflow\ChildWorkflowOptions::new()->withTaskQueue(Workflow::getInfo()->taskQueue),
        )->run();
    }
}

#[WorkflowInterface]
class ChildWorkflow
{
    #[WorkflowMethod('ChildWorkflow')]
    public function run()
    {
        throw new ApplicationFailure('Test message', 'TestError', true, EncodedValues::fromValues([['foo' => 'bar']]));
    }
}

// class FeatureChecker
// {
//     #[Check]
//     public static function check(#[Stub('MainWorkflow')] WorkflowStubInterface $stub): void
//     {
//         try {
//             $stub->getResult();
//             throw new \Exception('Expected exception');
//         } catch (WorkflowFailedException $e) {
//             Assert::same($e->getWorkflowType(), 'MainWorkflow');

//             /** @var ChildWorkflowFailure $previous */
//             $previous = $e->getPrevious();
//             Assert::isInstanceOf($previous, ChildWorkflowFailure::class);
//             Assert::same($previous->getWorkflowType(), 'ChildWorkflow');

//             /** @var ApplicationFailure $failure */
//             $failure = $previous->getPrevious();
//             Assert::isInstanceOf($failure, ApplicationFailure::class);
//             Assert::contains($failure->getOriginalMessage(), 'Test message');
//             Assert::same($failure->getType(), 'TestError');
//             Assert::same($failure->isNonRetryable(), true);
//             Assert::same($failure->getDetails()->getValue(0, 'array'), ['foo' => 'bar']);
//         }
//     }
// }
