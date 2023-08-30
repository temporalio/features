namespace Temporalio.Features.Harness;

/// <summary>
/// Feature type with directory.
/// </summary>
/// <param name="FeatureType">Type for the feature.</param>
public record PreparedFeature(Type FeatureType)
{
    /// <summary>
    /// Entire set of implementations of <see cref="IFeature" /> across the
    /// assemblies.
    /// </summary>
    public static readonly List<PreparedFeature> AllFeatures =
        // All types that are not abstract but implement IFeature
        AppDomain.CurrentDomain.GetAssemblies().
            SelectMany(a => a.GetTypes()).
            Where(t => !t.IsAbstract && typeof(IFeature).IsAssignableFrom(t)).
            Select(t => new PreparedFeature(t)).
            ToList();

    /// <summary>
    /// Gets the relative directory of the feature.
    /// </summary>
    public string Dir { get; } = FeatureType.Namespace!.Replace('.', '/');
}