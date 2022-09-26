package io.temporal.sdkfeatures;

import io.temporal.common.metadata.POJOWorkflowImplMetadata;

public class PreparedFeature {

  static PreparedFeature[] ALL = PreparedFeature.prepareFeatures(
      activity.retry_on_error.feature.Impl.class,
      activity.cancel_try_cancel.feature.Impl.class,
      continue_as_new.continue_as_same.feature.Impl.class,
      data_converter.binary.feature.Impl.class,
      data_converter.empty.feature.Impl.class,
      query.successful_query.feature.Impl.class,
      query.timeout_due_to_no_active_workers.feature.Impl.class,
      signal.external.feature.Impl.class
  );

  @SafeVarargs
  static PreparedFeature[] prepareFeatures(Class<? extends Feature>... classes) {
    var ret = new PreparedFeature[classes.length];
    for (int i = 0; i < classes.length; i++) {
      ret[i] = new PreparedFeature(classes[i]);
    }
    return ret;
  }

  final Class<? extends Feature> factoryClass;
  public final POJOWorkflowImplMetadata metadata;
  public final String dir;

  PreparedFeature(Class<? extends Feature> factoryClass) {
    this.factoryClass = factoryClass;
    this.metadata = POJOWorkflowImplMetadata.newInstance(factoryClass);
    // Directory is the package, but slashes instead of dots. We use string
    // instead of nio Path because we don't want platform-specific separator.
    dir = factoryClass.getPackageName().replace('.', '/');
  }

  Feature newInstance() {
    try {
      return factoryClass.getDeclaredConstructor().newInstance();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }
}
