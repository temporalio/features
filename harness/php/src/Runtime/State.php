<?php

declare(strict_types=1);

namespace Harness\Runtime;

use Harness\Input\Command;

final class State
{
    /** @var array<Feature> */
    public array $features = [];

    /** @var non-empty-string */
    public string $namespace;

    /** @var non-empty-string */
    public string $address;

    public function __construct(
        public readonly Command $command,
    ) {
        $this->namespace = $command->namespace ?? 'default';
        $this->address = $command->address ?? 'localhost:7233';
    }

    /**
     * Iterate over all the Workflows.
     *
     * @return \Traversable<Feature, class-string>
     */
    public function workflows(): \Traversable
    {
        foreach ($this->features as $feature) {
            foreach ($feature->workflows as $workflow) {
                yield $feature => $workflow;
            }
        }
    }

    /**
     * Iterate over all the Activities.
     *
     * @return \Traversable<Feature, class-string>
     */
    public function activities(): \Traversable
    {
        foreach ($this->features as $feature) {
            foreach ($feature->activities as $workflow) {
                yield $feature => $workflow;
            }
        }
    }

    /**
     * Iterate over all the Checks.
     *
     * @return \Traversable<Feature, array{class-string, non-empty-string}>
     */
    public function checks(): \Traversable
    {
        foreach ($this->features as $feature) {
            foreach ($feature->checks as $check) {
                yield $feature => $check;
            }
        }
    }

    /**
     * @param class-string $class
     * @param non-empty-string $method
     */
    public function addCheck(\Harness\Input\Feature $inputFeature, string $class, string $method): void
    {
        $this->getFeature($inputFeature)->checks[] = [$class, $method];
    }

    /**
     * @param class-string $class
     */
    public function addWorkflow(\Harness\Input\Feature $inputFeature, string $class): void
    {
        $this->getFeature($inputFeature)->workflows[] = $class;
    }

    /**
     * @param class-string $class
     */
    public function addActivity(\Harness\Input\Feature $inputFeature, string $class): void
    {
        $this->getFeature($inputFeature)->activities[] = $class;
    }

    private function getFeature(\Harness\Input\Feature $feature): Feature
    {
        return $this->features[$feature->namespace] ??= new Feature($feature->taskQueue);
    }
}