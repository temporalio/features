namespace Temporalio.Features.Harness;

using System.CommandLine;
using System.CommandLine.Invocation;
using Temporalio.Client;

/// <summary>
/// Main application that can parse args and run command.
/// </summary>
public static class App
{
    private static readonly Option<string> serverOption = new(
        name: "--server",
        description: "The host:port of the server")
    { IsRequired = true };

    private static readonly Option<string> directServerOption = new(
        name: "--direct-server",
        description: "The host:port of the server, bypassing the temporal-features-test-proxy");

    private static readonly Option<string> namespaceOption = new(
        name: "--namespace",
        description: "The namespace to use")
    { IsRequired = true };

    private static readonly Option<FileInfo?> clientCertPathOption = new(
        name: "--client-cert-path",
        description: "Path to a client certificate for TLS");

    private static readonly Option<FileInfo?> clientKeyPathOption = new(
        name: "--client-key-path",
        description: "Path to a client key for TLS");

    private static readonly Option<string> summaryUriOption = new(
        name: "--summary-uri",
        description: "Where to stream the test summary JSONL (not implemented)");

    private static readonly Option<string> proxyControlUriOption = new(
        name: "--proxy-control-uri",
        description: "URI for simulating network outages with temporal-features-test-proxy");

    private static readonly Argument<List<(string, string)>> featuresArgument = new(
        name: "features",
        parse: result => result.Tokens.Select(token =>
        {
            var pieces = token.Value.Split(':', 2);
            if (pieces.Length != 2)
            {
                throw new ArgumentException("Feature must be dir + ':' + task queue");
            }

            return (pieces[0], pieces[1]);
        }).ToList(),
        description: "Features as dir + ':' + task queue")
    { Arity = ArgumentArity.OneOrMore };

    /// <summary>
    /// Run this harness with the given args.
    /// </summary>
    /// <param name="args">CLI args.</param>
    /// <returns>Task for completion.</returns>
    public static Task RunAsync(string[] args) => CreateCommand().InvokeAsync(args);

    private static Command CreateCommand()
    {
        var cmd = new RootCommand(".NET features harness");
        cmd.AddOption(serverOption);
        cmd.AddOption(directServerOption);
        cmd.AddOption(namespaceOption);
        cmd.AddOption(clientCertPathOption);
        cmd.AddOption(clientKeyPathOption);
        cmd.AddOption(proxyControlUriOption);
        cmd.AddArgument(featuresArgument);
        cmd.SetHandler(RunCommandAsync);
        return cmd;
    }

    private static async Task RunCommandAsync(InvocationContext ctx)
    {
        // Create logger factory
        using var loggerFactory = LoggerFactory.Create(builder => builder.AddSimpleConsole(
            options =>
            {
                options.IncludeScopes = true;
                options.SingleLine = true;
                options.TimestampFormat = "HH:mm:ss ";
            }));
        var logger = loggerFactory.CreateLogger(typeof(App));

        // Connect a client
        var clientOptions =
            new TemporalClientConnectOptions(ctx.ParseResult.GetValueForOption(serverOption)!)
            {
                Namespace = ctx.ParseResult.GetValueForOption(namespaceOption)!,
                Tls = ctx.ParseResult.GetValueForOption(clientCertPathOption) is not { } certPath
                    ? null
                    : new()
                    {
                        ClientCert = File.ReadAllBytes(certPath.FullName),
                        ClientPrivateKey = File.ReadAllBytes(
                            ctx.ParseResult.GetValueForOption(clientKeyPathOption)?.FullName ??
                            throw new ArgumentException("Missing key with cert"))
                    }
            };

        // Go over each feature, calling the runner for it
        var failureCount = 0;
        foreach (var (dir, taskQueue) in ctx.ParseResult.GetValueForArgument(featuresArgument))
        {
            var feature =
                PreparedFeature.AllFeatures.SingleOrDefault(feature => feature.Dir == dir) ??
                throw new InvalidOperationException($"Unable to find feature for dir {dir}");
            try
            {
                await new Runner(clientOptions, taskQueue, feature, loggerFactory).RunAsync(
                    ctx.GetCancellationToken());
            }
            catch (TestSkippedException e)
            {
                logger.LogInformation("Feature {Feature} skipped: {Reason}", feature.Dir,
                    e.Message);
            }
            catch (Exception e)
            {
                logger.LogError(e, "Feature {Feature} failed", feature.Dir);
                failureCount++;
            }
        }

        if (failureCount > 0)
        {
            throw new InvalidOperationException($"{failureCount} feature(s) failed");
        }

        logger.LogInformation("All features passed");
    }
}
