<?php

declare(strict_types=1);

namespace Harness;

use Harness\Attribute\Check;
use Harness\Input\Command;
use Harness\Input\Feature;
use Harness\Runtime\State;
use Temporal\Activity\ActivityInterface;
use Temporal\DataConverter\PayloadConverterInterface;
use Temporal\Workflow\WorkflowInterface;

final class RuntimeBuilder
{
    public static function createState(array $argv, string $workDir): State
    {
        $command = Command::fromCommandLine($argv);
        $runtime = new State($command, \dirname(__DIR__), $workDir);

        $featuresDir = \dirname(__DIR__, 3) . '/features/';
        foreach (self::iterateClasses($featuresDir, $command) as $feature => $class) {
            # Register Workflow
            $class->getAttributes(WorkflowInterface::class) === [] or $runtime
                ->addWorkflow($feature, $class->getName());

            # Register Activity
            $class->getAttributes(ActivityInterface::class) === [] or $runtime
                ->addActivity($feature, $class->getName());

            # Register Converters
            $class->implementsInterface(PayloadConverterInterface::class) and $runtime
                ->addConverter($feature, $class->getName());

            # Register Check
            foreach ($class->getMethods() as $method) {
                $method->getAttributes(Check::class) === [] or $runtime
                    ->addCheck($feature, $class->getName(), $method->getName());
            }
        }

        return $runtime;
    }

    public static function init(): void
    {
        \ini_set('display_errors', 'stderr');
        include 'vendor/autoload.php';

        \spl_autoload_register(static function (string $class): void {
            if (\str_starts_with($class, 'Harness\\')) {
                $file = \str_replace('\\', '/', \substr($class, 8)) . '.php';
                $path = __DIR__ . '/' . $file;
                \is_file($path) and require $path;
            }
        });
    }

    /**
     * @param non-empty-string $featuresDir
     * @return iterable<Feature, \ReflectionClass>
     */
    private static function iterateClasses(string $featuresDir, Command $run): iterable
    {
        foreach ($run->features as $feature) {
            foreach (ClassLocator::loadClasses($featuresDir . $feature->dir, $feature->namespace) as $class) {
                yield $feature => new \ReflectionClass($class);
            }
        }
    }
}