<?php

declare(strict_types=1);

namespace Harness\Feature\Signal\PreventClose;

use Harness\Attribute\Check;
use Harness\Attribute\Stub;
use Harness\Exception\SkipTest;
use Temporal\Client\WorkflowStubInterface;
use Temporal\Exception\Client\WorkflowNotFoundException;
use Temporal\Workflow;
use Temporal\Workflow\SignalMethod;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;
use Webmozart\Assert\Assert;

#[WorkflowInterface]
class FeatureWorkflow
{
    private array $values = [];

    #[WorkflowMethod('Workflow')]
    public function run()
    {
        // Non-deterministic hack
        $replay = Workflow::isReplaying();

        yield Workflow::await(fn(): bool => $this->values !== []);

        // Add some blocking lag 300ms
        \usleep(300_000);

        return [$this->values, $replay];
    }

    #[SignalMethod('add')]
    public function add(int $arg)
    {
        $this->values[] = $arg;
    }
}

class FeatureChecker
{
    #[Check]
    public static function checkSignalOutOfExecution(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $stub->signal('add', 1);
        \usleep(1_500_000); // Wait 1.5s to workflow complete
        try {
            $stub->signal('add', 2);
            throw new \Exception('Workflow is not completed after the first signal.');
        } catch (WorkflowNotFoundException) {
            // false means the workflow was not replayed
            Assert::same($stub->getResult()[0], [1]);
            Assert::same($stub->getResult()[1], false, 'The workflow was not replayed');
        }
    }

    #[Check]
    public static function checkPreventClose(
        #[Stub('Workflow')] WorkflowStubInterface $stub,
    ): void {
        $stub->signal('add', 1);

        // Wait that the first signal is processed
        \usleep(200_000);

        // Add signal while WF is completing
        $stub->signal('add', 2);

        Assert::same($stub->getResult()[0], [1, 2], 'Both signals were processed');

        // todo: Find a better way
        // Assert::same($stub->getResult()[1], true, 'The workflow was replayed');
    }
}
