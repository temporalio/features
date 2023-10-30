namespace Temporalio.Features.Harness;

/// <summary>
/// Exception used to skip a feature.
/// </summary>
public class TestSkippedException : Exception
{
    /// <summary>
    /// Create exception.
    /// </summary>
    /// <param name="message">Reason for skipping.</param>
    public TestSkippedException(string message)
        : base(message)
    {
    }
}